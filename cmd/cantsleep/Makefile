BINARY=cantsleep
SOURCEDIR=../../tray
LIBDIR=.
SOURCES := $(shell find $(SOURCEDIR) $(LIBDIR) -name '*.go' -o -name '*.m' -o -name '*.h')

run: $(BINARY)
	./$(BINARY)

$(BINARY): $(SOURCES)
	go build -o $(BINARY)
	cp $(BINARY) CantSleep.app/Contents/MacOS/

clean:
	rm -f $(BINARY)