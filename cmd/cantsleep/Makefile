BINARY=cantsleep
SOURCEDIR=.
SOURCES := $(shell find $(SOURCEDIR) -name '*.go')

run: $(BINARY)
	./$(BINARY)

$(BINARY): $(SOURCES)
	go build -o $(BINARY)

clean:
	rm -f $(BINARY)