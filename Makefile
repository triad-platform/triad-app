SHELL := /bin/bash

.PHONY: up down ps logs smoke e2e

up:
	docker compose -f infra/compose/docker-compose.yml up -d

down:
	docker compose -f infra/compose/docker-compose.yml down -v

ps:
	docker compose -f infra/compose/docker-compose.yml ps

logs:
	docker compose -f infra/compose/docker-compose.yml logs -f --tail=200

smoke:
	@echo "Postgres:" && docker exec pulsecart-postgres pg_isready -U pulsecart
	@echo "Redis:" && docker exec pulsecart-redis redis-cli ping
	@echo "NATS:" && curl -sf http://localhost:8222/healthz >/dev/null && echo OK

e2e:
	./scripts/e2e-local.sh
