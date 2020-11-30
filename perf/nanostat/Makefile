regenerate-testdata:
	go install .
	cd testdata ; nanostat --iqrr nanobench_old.json  nanobench_new.json > iqrr.golden
	cd testdata ; nanostat --all  nanobench_old.json  nanobench_new.json > all.golden