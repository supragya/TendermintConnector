PROTOC=protoc
PROTOLOC=protocols/tmDataTransferProtocolv1
GO=go
GOBUILD=$(GO) build
BINDIR=build
BINCLI=tendermint_connector
BIN=$(BINDIR)/$(BINCLI)
INSTALLLOC=/usr/local/bin/$(BINCLI)

all:
	$(PROTOC) --go_out=. $(PROTOLOC)/*.proto
	$(GOBUILD) -o $(BIN)

clean:
	rm $(PROTOLOC)/*.go
	rm -rf $(BINDIR)/*

install:
	cp $(BIN) $(INSTALLLOC)

uninstall:
	rm $(INSTALLLOC)