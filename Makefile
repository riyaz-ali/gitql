#        _ _        _
#   __ _(_| |_ __ _| |
#  / _` | | __/ _` | |
# | (_| | | || (_| | |
#  \__, |_|\__\__, |_| - sql interface to git
#  |___/         |_|


LIBSRC = $(call rwildcard, gitql, *.go)
.build/libgitql.so: $(LIBSRC)
	$(call log, $(CYAN), "building shared library for extension")
	@go build -buildmode=c-shared -o $@ ./lib/shared
	$(call log, $(GREEN), "built libgitql.so")

CGO_LDFLAGS = --unresolved-symbols=ignore-in-object-files
ifeq ($(shell uname -s),Darwin)
	CGO_LDFLAGS = -undefined dynamic_lookup
endif

.build/gitql: $(wildcard *.go) $(LIBSRC) .build/libsqlite3.a
	$(call log, $(CYAN), "building git-sql executable")
	@LIBRARY_PATH=$(PWD)/.build/ CPATH=$(PWD)/.build/sqlite3 CGO_LDFLAGS="${CGO_LDFLAGS}" go build -o $@ -tags static,libsqlite3 .
	$(call log, $(GREEN), "built gitql executable")

# ========================================
## Targets to compile static build of sqlite3 for cli
# ========================================
SQLITE_CC     = gcc
SQLITE_FLAGS += -DSQLITE_LIKE_DOESNT_MATCH_BLOBS -DSQLITE_OMIT_DEPRECATED

.build/sqlite3/sqlite3.c:
	$(call log, $(CYAN), "downloading sqlite3 amalgamation source v3.33.0")
	$(eval SQLITE_DOWNLOAD_DIR = $(shell mktemp -d))
	@curl -sSLo $(SQLITE_DOWNLOAD_DIR)/sqlite3.zip https://www.sqlite.org/2020/sqlite-amalgamation-3330000.zip
	$(call log, $(GREEN), "downloaded sqlite3 amalgamation source v3.33.0")
	$(call log, $(CYAN), "unzipping to $(SQLITE_DOWNLOAD_DIR)")
	@(cd $(SQLITE_DOWNLOAD_DIR) && unzip sqlite3.zip > /dev/null)
	$(call log, $(CYAN), "moving to .build/sqlite3")
	@rm -rf .build/sqlite3 > /dev/null
	@mkdir -p .build/sqlite3
	@mv $(SQLITE_DOWNLOAD_DIR)/sqlite-amalgamation-3330000/* .build/sqlite3

.build/libsqlite3.o: .build/sqlite3/sqlite3.c
	$(call log, $(CYAN), "building libsqlite3.o object file")
	@$(SQLITE_CC) -Os -Wall -c -o $@ $(SQLITE_FLAGS) .build/sqlite3/sqlite3.c $(SQLITE_LIBS)
	$(call log, $(GREEN), "built libsqlite3.o")

.build/libsqlite3.a: .build/libsqlite3.o
	$(call log, $(CYAN), "building libsqlite3.a code archive")
	@ar rcs $@ .build/libsqlite3.o
	$(call log, $(GREEN), "built libsqlite3.a")

clean:
	$(call log, $(YELLOW), "nuking .build/")
	@-rm -rf .build/

# ========================================
# some utility methods

# ASCII color codes that can be used with functions that output to stdout
RED		:= 1;31
GREEN	:= 1;32
ORANGE	:= 1;33
YELLOW	:= 1;33
BLUE	:= 1;34
PURPLE	:= 1;35
CYAN	:= 1;36

# log:
#	print out $2 to stdout using $1 as ASCII color codes
define log
	@printf "\033[$(strip $1)m-- %s\033[0m\n" $2
endef

# recursive wildcard
rwildcard = $(foreach d,$(wildcard $(1:=/*)),$(call rwildcard,$d,$2) $(filter $(subst *,%,$2),$d))