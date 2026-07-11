.PHONY: bootstrap up down test test-api test-web check-contracts verify

bootstrap:
	pnpm install
	cd apps/api && go mod download

up:
	docker compose up -d

down:
	docker compose down

test: test-api test-web

test-api:
	cd apps/api && go test ./...

test-web:
	pnpm --dir apps/web lint
	pnpm --dir apps/web exec tsc --noEmit

check-contracts:
	@echo "Contract validation will be implemented in Iteration 00/01."

verify: test check-contracts
	docker compose config