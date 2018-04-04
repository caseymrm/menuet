BINARY=cantsleep
SOURCEDIR=.
SOURCES := $(shell find $(SOURCEDIR) -name '*.go' -o -name '*.c' -o -name '*.m') Makefile

run: $(BINARY)
	./$(BINARY)

$(BINARY): $(SOURCES)
	go build -o $(BINARY)

clean:
	rm -f $(BINARY)