BINARY=Weather.app/Contents/MacOS/weather
SOURCEDIR=. ../..
SOURCES := $(shell find $(SOURCEDIR) -name '*.go' -o -name '*.m' -o -name '*.h') Makefile

run: $(BINARY)
	./$(BINARY)

$(BINARY): $(SOURCES)
	go build -o $(BINARY)

clean:
	rm -f $(BINARY)