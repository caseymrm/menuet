BINARY=helloworld
SOURCEDIR=.
SOURCES := $(shell find $(SOURCEDIR) -name '*.go') Makefile

run: $(BINARY)
	./$(BINARY)

$(BINARY): $(SOURCES)
	go build -o $(BINARY)

clean:
	rm -f $(BINARY)