# This is a shared Makefile for building menuet apps. 
# To use it, create a Makefile in your applications directory, set the name of the app, and include this file. 
# For example:
#
#   BINARY=Weather.app/Contents/MacOS/weather
#   include $(GOPATH)/src/github.com/caseymrm/menuet/menuet.mk

SOURCEDIRS=$(abspath $(dir $(MAKEFILE_LIST)))
SOURCES := $(shell find $(SOURCEDIRS) -name '*.go' -o -name '*.m' -o -name '*.h' -o -name '*.mk' -o -name Makefile)

run: $(BINARY)
	./$(BINARY)

$(BINARY): $(SOURCES)
	go build -o $(BINARY)

clean:
	rm -f $(BINARY)