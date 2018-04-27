
configmap-reload:
	go install github.com/jimmidyson/configmap-reload

push_configmap_reload: configmap-reload
	./build_configmap_reload_release
	pushk configmap-reload
