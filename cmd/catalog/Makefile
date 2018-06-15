BINARY=Catalog.app/Contents/MacOS/catalog
SOURCEDIR=. ../..
SOURCES := $(shell find $(SOURCEDIR) -name '*.go' -o -name '*.m' -o -name '*.h') Makefile

run: $(BINARY)
	./$(BINARY)

$(BINARY): $(SOURCES)
	go build -o $(BINARY)

clean:
	rm -f $(BINARY)