#!/bin/bash

# mian文件全部build测试代码语法
for i in $(ls *.go | grep -v _test.go)
do
	go build -o targer $i && echo $i
done

rm -f targer

