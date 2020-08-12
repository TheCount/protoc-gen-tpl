#!/bin/sh

protoc --tpl_out=template=acme.txt.tpl,msgopt=example.acme_option,out=acme.txt:. acme.proto
protoc --tpl_out=template=acme.txt.tpl,msgopt=example.acme_option,extra=extra.json,out=acme_with_raw.txt:. acme.proto
