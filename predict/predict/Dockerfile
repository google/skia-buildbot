FROM gcr.io/skia-public/basealpine:3.7

COPY . /

ENTRYPOINT ["/usr/local/bin/predict"]
CMD ["--resources_dir=/usr/local/share/predict/", "--logtostderr", "--git_repo_dir=/tmp", "--namespace=predict-localhost", "--project=skia-public"]
