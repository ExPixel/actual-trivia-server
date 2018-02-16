#TODO Fix use the database credentials.
DATABASE=postgres://__user__:__password__@localhost:5432/__database__?sslmode=disable
MIGRATION_DIR=migrations

ifeq ($(dir),down)
	ifeq ($(times),)
		times="1"
	endif
endif

.PHONY: default
default: install

test: install
	go test -v ./...

# #FIXME This only works if the package gets installed and
# properly placed on the path somehow so I'll need to make some changes
# If I really want this to be consistent and portable.
run: install
	${GOPATH}/bin/trivia-server -level debug

# #FIXME Maybe I should be getting other things things in here too like vendor dependencies
# Or maybe I can add another target like make vdeps or something to do that instead.
deps:
	dep ensure -v

install:
	go install -v ./...

# Use make migrate dir={up/down} [times=N]
migrate: guard-dir
	migrate -path $(MIGRATION_DIR) -database $(DATABASE) $(dir) $(times)

# Used for creating a new migration. Set name={migration name}
create-migration: guard-name
	migrate create -ext pgsql -dir $(MIGRATION_DIR) $(name)

# Makes sure that a variable is defined.
guard-%:
	@ if [ "${${*}}" = "" ]; then \
		echo "Environment variable $* not set"; \
		exit 1; \
	fi
