# usage:
# make build v=0.4 is=latest push=true
build:
	docker build -t pema:${v} .
	docker tag pema:${v} ghcr.io/vadasambar/pema:${v}

	@if [ "${is}" = "latest" ]; then \
		docker tag pema:$v ghcr.io/vadasambar/pema:latest ; \
	fi

	@if [ "${push}" = "true" ]; then \
		docker push ghcr.io/vadasambar/pema:${v} ; \
		if [ "${is}" = "latest" ]; then \
			docker push ghcr.io/vadasambar/pema:latest ; \
		fi ; \
	fi