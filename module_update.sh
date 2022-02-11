#!/bin/bash

# 该脚本用来自动更新module

go mod tidy

match_required=$(cat go.mod | grep -zoE "\((.*?)\)" | awk -F ' ' '{print $1}' | awk '{if($1>1){print $1}}')

for i in $match_required;do go get -u "$i";done

go mod tidy
