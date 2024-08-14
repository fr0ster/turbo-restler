module github.com/fr0ster/turbo-restler

go 1.22.5

require (
	github.com/bitly/go-simplejson v0.5.1
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/stretchr/testify v1.9.0
)

require golang.org/x/sys v0.0.0-20220715151400-c0bba94af5f8 // indirect

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fr0ster/turbo-signer v0.0.0-20240814075010-ef27de27a887
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sirupsen/logrus v1.9.3
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract [v0.1.0, v0.1.41] // Retract all from v0.1.0 to v0.1.41, this tags were made by mistake
