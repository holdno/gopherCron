#!/bin/sh

cp -r _build gophercron-linux-amd64
tar -zcvf gophercron-linux-amd64.tar.gz gophercron-linux-amd64
zip gophercron-linux-amd64.zip -r gophercron-linux-amd64
rm -rf gophercron-linux-amd64