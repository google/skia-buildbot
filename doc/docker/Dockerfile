FROM debian:testing-slim
RUN apt-get update && apt-get install -y \
  git

COPY docserver  /usr/local/bin/docserver
COPY index.html /usr/local/share/docserver/templates/index.html
COPY core.js    /usr/local/share/docserver/res/js/core.js
COPY logo.png   /usr/local/share/docserver/res/img/logo.png
COPY docs.css   /usr/local/share/docserver/res/css/docs.css

EXPOSE 8000 20000

CMD ["/usr/local/bin/docserver", "--logtostderr", "--resources_dir=/usr/local/share/docserver/"]
#CMD ["bash"]
