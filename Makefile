BUILD_DIR ?= build
CFLAGS := $(shell pkg-config fuse3 --cflags) -Wall -s -O2
LIBS := $(shell pkg-config fuse3 --libs)
IMAGE ?= dummy-fuse-csi
IMAGE_TAG ?= latest
VERSION ?= $(shell git describe --long --tags --dirty --always)
CSI_GOLDFLAGS := "-w -s -X 'github.com/gman0/dummy-fuse-csi/csi/internal/version.Version=${VERSION}'"
WORKLOAD_GOLDFLAGS := "-w -s"

$(shell mkdir -p $(BUILD_DIR))

all: dummy-fuse dummy-fuse-csi dummy-fuse-workload

dummy-fuse: fs/dummy-fuse.c $(BUILD_DIR)/version.o
	gcc $(CFLAGS) $(LIBS) $^ -o $(BUILD_DIR)/$@

$(BUILD_DIR)/version.c:
	@printf "%s" 'const char *dummy_version = "$(VERSION)";' > $(BUILD_DIR)/version.c

$(BUILD_DIR)/version.o: $(BUILD_DIR)/version.c
	gcc -c -o $(@:.c=.o) $<

dummy-fuse-csi:
	cd csi; CGO_ENABLED=0 go build -ldflags $(CSI_GOLDFLAGS) -o ../$(BUILD_DIR)/$@ cmd/main.go

dummy-fuse-workload:
	cd workload; CGO_ENABLED=0 go build -ldflags $(WORKLOAD_GOLDFLAGS) -o ../$(BUILD_DIR)/$@ cmd/main.go

image: dummy-fuse dummy-fuse-csi dummy-fuse-workload
	podman build -f ./Dockerfile $(BUILD_DIR) -t $(IMAGE):$(IMAGE_TAG)

generate-compile-flags:
	$(shell echo $(CFLAGS) | tr " " "\n" > compile_flags.txt)

clean:
	rm -rf $(BUILD_DIR)

.PHONY: all clean dummy-fuse dummy-fuse-csi generate-compile-flags
