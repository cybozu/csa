.PHONY: help
help:
	@echo "Choose following commands:"
	@echo "    git-csa: install git-csa tool"

.PHONY: git-csa
git-csa:
	cd git-csa; go install .
