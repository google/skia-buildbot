FROM gcr.io/skia-public/basealpine:3.7

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/power-controller"]
CMD [\
	"--authorized_email=jumphost@skia-buildbots.google.com.iam.gserviceaccount.com", \
	"--logtostderr", \
	"--port=:8000", \
	"--powercycle_config=/etc/powercycle/powercycle-internal-01.json5", \
	"--powercycle_config=/etc/powercycle/powercycle-linux-01.json5", \
	"--powercycle_config=/etc/powercycle/powercycle-rpi-01.json5", \
	"--powercycle_config=/etc/powercycle/powercycle-win-02.json5", \
	"--powercycle_config=/etc/powercycle/powercycle-win-03.json5", \
	"--prom_port=:20000", \
	"--resources_dir=/usr/local/share/power-controller/", \
]