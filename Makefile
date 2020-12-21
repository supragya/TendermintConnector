PROTOC=protoc
PROTOLOC=protocols/tmDataTransferProtocolv1
GO=go
GOBUILD=$(GO) build
BINDIR=build
BINCLI=TendermintConnector
INSTALLLOC=/usr/local/bin/$(BINCLI)
RELEASE=$(shell git describe --tags --abbrev=0)
BUILDCOMMIT=$(shell git rev-parse HEAD)
BUILDLINE=$(shell git rev-parse --abbrev-ref HEAD)
CURRENTTIME=$(shell date -u '+%d-%m-%Y_%H-%M-%S')@UTC

release:
	$(PROTOC) --go_out=. $(PROTOLOC)/*.proto
	$(GOBUILD) -ldflags="\
						-X github.com/supragya/TendermintConnector/cmd.compilationChain=iris \
						-X github.com/supragya/TendermintConnector/version.applicationVersion=$(RELEASE)-release \
						-X github.com/supragya/TendermintConnector/version.buildCommit=$(BUILDLINE)@$(BUILDCOMMIT) \
						-X github.com/supragya/TendermintConnector/version.buildTime=$(CURRENTTIME) \
						-linkmode=external" \
				-o $(BINDIR)/iris_connector 
	$(GOBUILD) -ldflags="\
						-X github.com/supragya/TendermintConnector/cmd.compilationChain=cosmos \
						-X github.com/supragya/TendermintConnector/version.applicationVersion=$(RELEASE)-release \
						-X github.com/supragya/TendermintConnector/version.buildCommit=$(BUILDLINE)@$(BUILDCOMMIT) \
						-X github.com/supragya/TendermintConnector/version.buildTime=$(CURRENTTIME) \
						-linkmode=external" \
				-o $(BINDIR)/cosmos_connector 

internal:
	$(PROTOC) --go_out=. $(PROTOLOC)/*.proto
	$(GOBUILD) -ldflags="\
						-X github.com/supragya/TendermintConnector/cmd.compilationChain=iris \
						-X github.com/supragya/TendermintConnector/version.applicationVersion=$(RELEASE)-internal \
						-X github.com/supragya/TendermintConnector/version.buildCommit=$(BUILDLINE)@$(BUILDCOMMIT) \
						-X github.com/supragya/TendermintConnector/version.buildTime=$(CURRENTTIME) \
						-linkmode=external" \
				-o $(BINDIR)/iris_connector 
	$(GOBUILD) -ldflags="\
						-X github.com/supragya/TendermintConnector/cmd.compilationChain=cosmos \
						-X github.com/supragya/TendermintConnector/version.applicationVersion=$(RELEASE)-internal \
						-X github.com/supragya/TendermintConnector/version.buildCommit=$(BUILDLINE)@$(BUILDCOMMIT) \
						-X github.com/supragya/TendermintConnector/version.buildTime=$(CURRENTTIME) \
						-linkmode=external" \
				-o $(BINDIR)/cosmos_connector 

clean:
	rm $(PROTOLOC)/*.go
	rm -rf $(BINDIR)/*

install:
	cp $(BIN) $(INSTALLLOC)

uninstall:
	rm $(INSTALLLOC)