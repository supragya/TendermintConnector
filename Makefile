PROTOC=protoc
PROTOLOC=protocols/tmDataTransferProtocolv1
GO=go
GOBUILD=$(GO) build
BINDIR=build
BINCLI=TendermintConnector
INSTALLLOC=/usr/local/bin/$(BINCLI)

all:
	$(PROTOC) --go_out=. $(PROTOLOC)/*.proto
	$(GOBUILD) -ldflags="-X github.com/supragya/TendermintConnector/cmd.compilationChain=iris -linkmode=external" -o $(BINDIR)/iris_connector 
	$(GOBUILD) -ldflags="-X github.com/supragya/TendermintConnector/cmd.compilationChain=cosmos -linkmode=external" -o $(BINDIR)/cosmos_connector

clean:
	rm $(PROTOLOC)/*.go
	rm -rf $(BINDIR)/*

install:
	cp $(BIN) $(INSTALLLOC)

uninstall:
	rm $(INSTALLLOC)