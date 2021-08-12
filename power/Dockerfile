FROM gcr.io/skia-public/basealpine:3.8

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/power-controller"]
CMD [\
	"--authorized_email=jumphost@skia-buildbots.google.com.iam.gserviceaccount.com", \
	"--port=:8000", \
	"--powercycle_config=/etc/powercycle/powercycle-internal-01.json5", \
	"--powercycle_config=/etc/powercycle/powercycle-rack-01.json5", \
	"--powercycle_config=/etc/powercycle/powercycle-rack-02.json5", \
	"--powercycle_config=/etc/powercycle/powercycle-rack-03.json5", \
	"--prom_port=:20000", \
	"--resources_dir=/usr/local/share/power-controller/", \
]