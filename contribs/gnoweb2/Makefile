# Variable Declarations
COMPONENTS_FOLDER := .

tools_run := go run -modfile ./gotools/go.mod
tools_install := go install -modfile ./gotools/go.mod

run_templ := $(tools_run) github.com/a-h/templ/cmd/templ
run_air := $(tools_run) github.com/air-verse/air
run_logname := go -C ./gotools run ./cmd/logname

# Tailwind config
output_css := ./assets/styles.css
input_css := ./input.css

#############
# Targets
#############
.PHONY: all generate fmt tailwind templ

# Install dependencies
all: generate

install: generate
	go install -v .

# Generate process
generate: tailwind templ
tailwind:
	npx tailwindcss -i $(input_css) -o $(output_css) --minify # tailwind
templ:
	$(run_templ) generate $(COMPONENTS_FOLDER) # templ

# Format process
fmt:
	go fmt ./...
	$(run_templ) fmt $(COMPONENTS_FOLDER)

 ###############################
 # Developments
 ###############################
.PHONY: dev dev.templ dev.server dev.tailwind deps

# This rule run the development dependencies in parallel.
dev:
	$(MAKE) -j 3 dev.templ dev.server dev.tailwind

# `Templ` in development mode
dev.templ:
	$(run_templ) generate -v --watch --proxy="http://localhost:9000" \
		--open-browser=false $(COMPONENTS_FOLDER) \
		2>&1 | $(run_logname) templ

# Go server in development mode
dev.server: | .cache
	$(run_air) \
		--build.cmd "go build -o .cache/serve" --build.bin ".cache/serve" \
		--build.delay "100" \
		--build.exclude_dir "gotools,node_modules" \
		--build.include_ext "go"  --build.stop_on_error "false" \
		--color.app "#00FF00" \
		--misc.clean_on_exit true \
		2>&1 | $(run_logname) gnoweb

# Tailwind CSS in development mode
dev.tailwind: node_modules
	npx tailwindcss -i $(input_css) -o $(output_css) --minify --watch \
		2>&1 | $(run_logname) tailwind

# Install deps for development purpose
deps:
	$(tools_install) github.com/a-h/templ/cmd/templ

## Dependency install
package-lock.json node_modules: package.json
	npm install && touch $@

.cache:; mkdir -p $@
