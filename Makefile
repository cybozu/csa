.PHONY: help
help:
	@echo "Choose following commands:"
	@echo "    git-csa    install git-csa tool"
	@echo "    test       test tools"

.PHONY: git-csa
git-csa:
	cd git-csa; go install .

.PHOEY: test
test:
	cd git-csa; go test ./...
