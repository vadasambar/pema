package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"context"
	"log"

	"github.com/antonmedv/expr"
	"github.com/mongodb-forks/digest"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.mongodb.org/atlas/mongodbatlas"
	"gopkg.in/yaml.v2"
)

const (
	ValueTypeString    = "string"
	ValueTypeCondition = "condition"
)

var (
	activeClusters *prometheus.GaugeVec
	settings       *Settings
)

type Tag struct {
	// Value is a string or Expr compatible expression.
	// Go templates is not supported
	Value interface{} `yaml:"value"`
	// // TODO: not supported yet. This is for
	// // supporting syntax other than Expr e.g., Go templates
	// Syntax string
	// // TODO: not supported yet. Allow using result of expressions as
	// // variables
	// LocalVariables []LocalVariable
	// To store actual value or condition frmo Value interface
	conditions []Condition
	value      string
	valueType  ValueType
}

type ValueType string

type Condition struct {
	If   string `yaml:"if"`
	Then string `yaml:"then"`
}

type Value string

type Settings struct {
	// not specifying yaml tag here leads to error when unmarshaling
	ProjectID string `yaml:"projectId"`
	// adding yaml tag here just for the consistency
	Tags map[string]*Tag `yaml:"tags"`
}

type Env struct {
	Cluster mongodbatlas.Cluster
}

func main() {
	const interval = 10
	var err error
	settings, err = readConfig()
	if err != nil {
		panic(err)
	}
	log.Println(*settings)
	for _, tag := range settings.Tags {
		log.Println("tag", *tag)
	}

	activeClusters = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "pema",
		Subsystem: "mongodb_atlas",
		Name:      "active_clusters",
		Help:      "Number of clusters active with similar name/label",
	},
		settings.getTagNames(),
	)
	prometheus.MustRegister(activeClusters)

	http.Handle("/metrics", promhttp.Handler())

	go func() {
		ticker := time.NewTicker(interval * time.Second)

		// register metrics in the background
		for range ticker.C {
			c, err := getClusters()
			if err != nil {
				log.Fatal(err)
			}
			err = setMetrics(c)
		}
	}()

	log.Println("Serving at 5000")
	log.Fatal(http.ListenAndServe(":5000", nil))

}

func (s Settings) getTagNames() []string {
	names := []string{}
	for k, _ := range s.Tags {
		names = append(names, k)
	}

	return names
}

func readConfig() (*Settings, error) {
	d, err := ioutil.ReadFile(os.Getenv("SETTINGS_PATH"))
	if err != nil {
		return nil, err
	}
	settings := &Settings{}
	if err := yaml.Unmarshal(d, settings); err != nil {
		return nil, err
	}

	for tag, v := range settings.Tags {
		var str string
		var ok bool
		settings.Tags[tag].conditions = []Condition{}
		// condition
		if str, ok = v.Value.(string); !ok {
			// conditions of the type map[interface{}]interface{}
			i, ok := v.Value.([]interface{})
			if !ok {
				return nil, fmt.Errorf("error loading conditions :(")
			}

			for _, conditions := range i {
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

					settings.Tags[tag].conditions = append(settings.Tags[tag].conditions, cond)
				}

			}
			settings.Tags[tag].valueType = ValueTypeCondition

			// normal expr string
		} else {
			settings.Tags[tag].valueType = ValueTypeString
			settings.Tags[tag].value = str
		}

	}

	return settings, nil

}

func getClusters() ([]mongodbatlas.Cluster, error) {
	t := digest.NewTransport(os.Getenv("ATLAS_PUBLIC_KEY"), os.Getenv("ATLAS_PRIVATE_KEY"))
	tc, err := t.Client()
	if err != nil {
		log.Fatalf(err.Error())
	}

	client := mongodbatlas.NewClient(tc)

	clusters, _, err := client.Clusters.List(context.Background(), settings.ProjectID, nil)
	if err != nil {
		return nil, err
	}
	return clusters, nil
}

func setMetrics(clusters []mongodbatlas.Cluster) error {
	promLabels := prometheus.Labels{}
	for _, cluster := range clusters {
		for k, v := range settings.Tags {
			switch v.valueType {
			case ValueTypeString:
				// p, err := expr.Compile(v.value, expr.Env(Env{}))

				// if err != nil {
				// 	panic(err)
				// }

				output, err := expr.Eval(v.value, Env{
					Cluster: cluster,
					// Foo: "bar",
				})
				// fmt.Println("output", output)
				if err != nil {
					panic(err)
				}
				promLabels[k] = output.(string)
			case ValueTypeCondition:
				for _, condition := range v.conditions {
					// p, err := expr.Compile(condition.If, expr.Env(Env{}))

					// if err != nil {
					// 	panic(err)
					// }

					output, err := expr.Eval(condition.If, Env{
						Cluster: cluster,
					})
					if err != nil {
						panic(err)
					}

					op := output.(bool)
					// fmt.Println("cluster name", cluster.Name, "condition", condition.If, "output", output, "op", op)
					if op == true {
						promLabels[k] = condition.Then
						continue
					}
				}
				// if key is not set i.e., no condition returns true
				// set it to empty
				if _, ok := promLabels[k]; !ok {
					promLabels[k] = ""
				}
			default:
				panic(fmt.Errorf("value type not set on the tag"))
			}

		}

		fmt.Println("promeLabels", promLabels)
		activeClusters.With(promLabels).Set(1)
	}
	return nil
}
