# This is a shared Makefile for building menuet apps. 
# To use it, create a Makefile in your applications directory, set the name of the app, and include this file. 
# For example:
#
#   BINARY=Weather.app/Contents/MacOS/weather
#   include $(GOPATH)/src/github.com/caseymrm/menuet/menuet.mk

ifndef APP
  $(error APP variable must be defined, e.g. APP=Hello World)
endif

SOURCEDIRS=$(abspath $(dir $(MAKEFILE_LIST)))
SOURCES := $(shell find $(SOURCEDIRS) -name '*.go' -o -name '*.m' -o -name '*.h' -o -name '*.mk' -o -name Makefile)

space :=
space +=
ESCAPED_APP=$(subst $(space),\$(space),$(APP))
EXECUTABLE=$(subst $(space),,$(APP))
BINARY=$(ESCAPED_APP).app/Contents/MacOS/$(EXECUTABLE)
PLIST=$(ESCAPED_APP).app/Contents/Info.plist

run: $(BINARY) $(PLIST)
	./$(BINARY)

$(BINARY): $(SOURCES)
	go build -o $(BINARY)

clean:
	rm -f $(BINARY) $(PLIST)

$(PLIST):
	echo "Generating plist..."
	@echo '<?xml version="1.0" encoding="UTF-8"?>' > $(PLIST)
	@echo '<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">' >> $(PLIST)
	@echo '<plist version="1.0">' >> $(PLIST)
	@echo '<dict>' >> $(PLIST)
	@echo '  <key>CFBundleExecutable</key>' >> $(PLIST)
	@echo '  <string>$(EXECUTABLE)</string>' >> $(PLIST)
	@echo '  <key>CFBundleIconFile</key>' >> $(PLIST)
	@echo '  <string>icon</string>' >> $(PLIST)
	@echo '  <key>CFBundleGetInfoString</key>' >> $(PLIST)
	@echo '  <string>$(APP)</string>' >> $(PLIST)
	@echo '  <key>CFBundleIdentifier</key>' >> $(PLIST)
	@echo '  <string>$(EXECUTABLE).menuet.caseymrm.github.com</string>' >> $(PLIST)
	@echo '  <key>CFBundleName</key>' >> $(PLIST)
	@echo '  <string>$(APP)</string>' >> $(PLIST)
	@echo '  <key>CFBundleShortVersionString</key>' >> $(PLIST)
	@echo '  <string>0.1</string>' >> $(PLIST)
	@echo '  <key>CFBundleInfoDictionaryVersion</key>' >> $(PLIST)
	@echo '  <string>6.0</string>' >> $(PLIST)
	@echo '  <key>CFBundlePackageType</key>' >> $(PLIST)
	@echo '  <string>APPL</string>' >> $(PLIST)
	@echo '  <key>IFMajorVersion</key>' >> $(PLIST)
	@echo '  <integer>0</integer>' >> $(PLIST)
	@echo '  <key>IFMinorVersion</key>' >> $(PLIST)
	@echo '  <integer>1</integer>' >> $(PLIST)
	@echo '  <key>NSHighResolutionCapable</key><true/>' >> $(PLIST)
	@echo '  <key>NSSupportsAutomaticGraphicsSwitching</key><true/>' >> $(PLIST)
	@echo '</dict>' >> $(PLIST)
	@echo '</plist>' >> $(PLIST)