#!/bin/bash

go list -f '{{join .Deps "\n"}}' ../... | sort | uniq | grep "[a-zA-Z0-9]\+\." > all_deps.txt
