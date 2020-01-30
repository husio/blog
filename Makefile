all: build

dev:
	hugo --buildDrafts --watch server

build:
	hugo --gc --minify --cleanDestinationDir
