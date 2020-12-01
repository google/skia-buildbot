build:
	go install .

test:
	go test .

regenerate-testdata:
	go install .
	cd testdata ; nanostat --iqrr nanobench_old.json  nanobench_new.json > iqrr.golden
	cd testdata ; nanostat --all  nanobench_old.json  nanobench_new.json > all.golden
	cd testdata ; nanostat --sort=name nanobench_old.json  nanobench_new.json > sort.golden
	cd testdata ; nanostat --test=ttest nanobench_old.json  nanobench_new.json > test.golden
	cd testdata ; nanostat nanobench_old.json  nanobench_old.json > nochange.golden