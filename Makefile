.PHONY: default
default: install

test: install-race
	go test -v ./...

# #FIXME This only works if the package gets installed and
# properly placed on the path somehow so I'll need to make some changes
# If I really want this to be consistent and portable.
run: install
	${GOPATH}/bin/trivia-server -level debug
run-race: install-race
	${GOPATH}/bin/trivia-server -level debug

debug: install
	dlv debug github.com/expixel/actual-trivia-server/cmd/trivia-server
debug-race: install-race
	dlv debug github.com/expixel/actual-trivia-server/cmd/trivia-server

# #FIXME Maybe I should be getting other things things in here too like vendor dependencies
# Or maybe I can add another target like make vdeps or something to do that instead.
deps:
	dep ensure -v

install:
	go install -v ./...

install-race:
	go install -race -v ./...

# Makes sure that a variable is defined.
guard-%:
	@ if [ "${${*}}" = "" ]; then \
		echo "Environment variable $* not set"; \
		exit 1; \
	fi
