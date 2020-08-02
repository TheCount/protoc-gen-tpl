#!/bin/sh

protoc --tpl_out=template=acme.txt.tpl,msgopt=example.acme_option,out=acme.txt:. acme.proto
