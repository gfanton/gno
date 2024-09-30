DEV_REMOTE ?= 127.0.0.1:8888

# Variable Declarations
COMPONENTS_FOLDER := ./components

tools_run := go run -modfile ./gotools/go.mod
tools_install := go install -modfile ./gotools/go.mod

run_templ := $(tools_run) github.com/a-h/templ/cmd/templ
run_air := $(tools_run) github.com/air-verse/air
run_logname := go -C ./gotools run ./cmd/logname

# Tailwind config
input_css := ./input.css
output_css := ./public/styles.css

# Esbuild config
input_js := $(COMPONENTS_FOLDER)/**/*.ts
output_js := ./public/js

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

# Run the development dependencies in parallel
dev:
	@echo "-- starting development tools"
	$(MAKE) -j 4 \
		dev.templ \
		dev.server \
		dev.esbuild \
		dev.tailwind

# `Templ` in development mode
dev.templ:
	$(run_templ) generate -v --watch --proxy="http://localhost:9000" \
		--open-browser=false $(COMPONENTS_FOLDER) \
		2>&1 | $(run_logname) templ

# Go server in development mode
dev.server: | .cache
	$(run_air) \
		--build.cmd "go build -o .cache/serve ./cmd/gnoweb" --build.bin ".cache/serve -remote=${DEV_REMOTE}" \
		--build.exclude_unchanged "true" \
		--build.exclude_dir "gotools,node_modules" \
		--build.exclude_regex '^\.#.*' \
		--build.include_ext "go" \
		--build.stop_on_error "false" \
		--build.rerun_delay "1000" \
		2>&1 | $(run_logname) gnoweb

# Tailwind CSS in development mode
dev.tailwind: node_modules | public
	npx postcss $(input_css) -o $(output_css) --watch \
		2>&1 | $(run_logname) postcss

# Tailwind CSS in development mode
dev.esbuild: node_modules | public
	npx esbuild $(input_js) --bundle --outdir=$(output_js) --watch \
		2>&1 | $(run_logname) esbuild

# Install deps for development purpose
deps:
	$(tools_install) github.com/a-h/templ/cmd/templ

## Dependency install
package-lock.json node_modules: package.json
	npm install && touch $@

clean:
	rm -rf public

fclean: clean
	rm -rf node_modules .cache

.cache:; mkdir -p $@
public:; mkdir -p $@
