TAGS :=

run: build
	./adspots

build:
	go build -v -tags "$(TAGS)" -o adspots ./cmd/adspots

clean:
	rm -f ./adspots

check:
	env GOEXPERIMENT=synctest go test -v -tags "$(TAGS)" ./t

.PHONY: build run clean
