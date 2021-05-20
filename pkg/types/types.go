package types

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/antonmedv/expr"
	"github.com/mongodb-forks/digest"
	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/atlas/mongodbatlas"
	"gopkg.in/yaml.v2"
)

const (
	// ValueTypeString is a string value type
	// e.g., value: "foo", value: "{Cluster.Name}"
	ValueTypeString = "string"
	// ValueTypeCondition is a conditional type of value
	// e.g, value:
	// 		 - if: Cluster.Name matches 'staging.*'
	// 		   then: staging
	// 		 - if: Cluster.Name matches 'production.*'
	// 		   then: production
	ValueTypeCondition = "condition"
)

// Tag holds data for 'tag' field in the settings
type Tag struct {
	// Value is a string or Expr compatible expression.
	// Go templates is not supported
	Value interface{} `yaml:"value"`
	// conditions is used to store
	// actual value or condition from Value interface
	conditions []Condition
	// value is used to store string value from value field
	value string
	// valueType denotes the type of value
	valueType ValueType
}

// ValueType denotes type of Value
// Supported Types: ValueTypeString, ValueTypeCondition
type ValueType string

// Condition is a conditional statement
// to set value
type Condition struct {
	If   string `yaml:"if"`
	Then string `yaml:"then"`
}

// Value denotes string metric value
type Value string

// Settings holds all the fields
// in the settings/config yaml
type Settings struct {
	// not specifying yaml tag here leads to error when unmarshaling
	ProjectID string `yaml:"projectId"`
	// adding yaml tag here just for the consistency
	Tags map[string]*Tag `yaml:"tags"`
}

// Exporter denotes instance of an exporter
// This is an overarching object initialized only once
type Exporter struct {
	// Settings read from settings yaml
	Settings *Settings
	// Metrics holds multiple metrics for processing
	// Used only for a single metric as of now
	Metrics []interface{}
}

// Env is environment for evaluating
// Expr statements
type Env struct {
	Cluster mongodbatlas.Cluster
}

// GetTagNames returns a list of tag names
// defined in the settings yaml
func (s *Settings) GetTagNames() []string {
	names := []string{}
	for k, _ := range s.Tags {
		names = append(names, k)
	}

	return names
}

// ReadSettings reads the settings from settings yaml
// and loads it into Settings struct
func (e *Exporter) ReadSettings() (*Settings, error) {
	d, err := ioutil.ReadFile(os.Getenv("SETTINGS_PATH"))
	if err != nil {
		return nil, err
	}
	s := &Settings{}
	if err := yaml.Unmarshal(d, s); err != nil {
		return nil, err
	}

	e.Settings = s
	for tag, v := range e.Settings.Tags {
		var str string
		var ok bool
		e.Settings.Tags[tag].conditions = []Condition{}
		// if condition is anything other than string e.g., if-then
		if str, ok = v.Value.(string); !ok {
			// list of if-then statements
			i, ok := v.Value.([]interface{})
			if !ok {
				return nil, fmt.Errorf("error loading conditions :(")
			}

			for _, conditions := range i {
				// content of if-then statements is of the type map[interface{}]interface{}
				// e.g.,
				// - if: true
				// 	 then: foo
				conds := conditions.(map[interface{}]interface{})
				cond := Condition{}
				for k, v := range conds {
					ks := k.(string)
					vs := v.(string)
					switch ks {
					case "if":
						cond.If = vs
					case "then":
						cond.Then = vs
					default:
						return nil, fmt.Errorf("%s is not supported for if-then", k)
					}

					e.Settings.Tags[tag].conditions = append(e.Settings.Tags[tag].conditions, cond)
				}

			}
			e.Settings.Tags[tag].valueType = ValueTypeCondition

			// normal expr string
		} else {
			e.Settings.Tags[tag].valueType = ValueTypeString
			e.Settings.Tags[tag].value = str
		}

	}

	log.Println("loaded settings")

	return e.Settings, nil

}

// GetClusters returns all the clusters matching a particular
// project ID (set in the settings yaml)
func (e *Exporter) GetClusters() ([]mongodbatlas.Cluster, error) {
	t := digest.NewTransport(os.Getenv("ATLAS_PUBLIC_KEY"), os.Getenv("ATLAS_PRIVATE_KEY"))
	tc, err := t.Client()
	if err != nil {
		log.Fatalf(err.Error())
	}

	client := mongodbatlas.NewClient(tc)

	clusters, _, err := client.Clusters.List(context.Background(), e.Settings.ProjectID, nil)
	if err != nil {
		return nil, err
	}
	return clusters, nil
}

// EvaluateLabels evaluates if-then conditions, Expr statements
// and creates prometheus labels out of them
func (e *Exporter) EvaluateLabels(cluster *mongodbatlas.Cluster) (prometheus.Labels, error) {
	promLabels := prometheus.Labels{}

	for k, v := range e.Settings.Tags {
		switch v.valueType {
		case ValueTypeString:

			output, err := expr.Eval(v.value, Env{
				Cluster: *cluster,
			})
			if err != nil {
				return nil, err
			}

			s, err := toString(output)
			if err != nil {
				return nil, fmt.Errorf("value: %v: %v", v.value, err)
			}

			promLabels[k] = s

		case ValueTypeCondition:
			for _, condition := range v.conditions {

				output, err := expr.Eval(condition.If, Env{
					Cluster: *cluster,
				})
				if err != nil {
					return nil, err
				}

				op, ok := output.(bool)
				if !ok {
					return nil, fmt.Errorf("output of '%s' has to be a bool", condition.If)
				}
				if op == true {

					s, err := toString(condition.Then)
					if err != nil {
						return nil, fmt.Errorf("condition: %v: %v", condition.Then, err)
					}

					promLabels[k] = s
					continue
				}
			}
			// if key is not set i.e., no condition returns true
			// set it to empty
			if _, ok := promLabels[k]; !ok {
				promLabels[k] = ""
			}
		default:
			return nil, fmt.Errorf("value type not set on the tag")
		}

	}

	return promLabels, nil
}

// SetMetrics is used to set value of a metric
func (e *Exporter) SetMetrics(clusters []mongodbatlas.Cluster) error {
	// Resets metric to reflect change in clusters
	// Not doing this requires pod restart
	for _, metric := range e.Metrics {
		// TODO: Support other metric types
		m, ok := metric.(*prometheus.GaugeVec)
		if !ok {
			return fmt.Errorf("could not typecast into gauge vector")
		}
		m.Reset()
	}

	for _, cluster := range clusters {
		for _, metric := range e.Metrics {
			promLabels, err := e.EvaluateLabels(&cluster)
			if err != nil {
				return err
			}
			// TODO: Support other metric types
			m, ok := metric.(*prometheus.GaugeVec)
			if !ok {
				return fmt.Errorf("could not typecast into gauge vector")
			}
			m.With(promLabels).Set(1)
		}
	}
	return nil
}

// toString uses a json.Marshal hack
// to convert anything to a string
func toString(i interface{}) (string, error) {
	str, ok := i.(string)
	if !ok {
		s, err := json.Marshal(i)
		if err != nil {
			return "", err
		}
		str = string(s)
	}

	return str, nil
}
