// Command seed-demo-courses is a dev-only load-testing helper for Stage 13
// (search/filters/sort/pagination). It inserts 40 demo courses spread
// across the 4 seeded categories with varied levels/access types and
// distinctive, searchable Russian titles/descriptions — enough data to
// exercise the GIN full-text index, category/level/access filters, every
// sort mode, and multi-page pagination.
//
// It is intentionally NOT a goose migration: this is disposable load-test
// data, not schema or data every environment must have. Run it manually
// against a dev database:
//
//	go run ./cmd/seed-demo-courses
package main

import (
	"context"
	"fmt"
	"log"

	"lms-backend/internal/config"
	"lms-backend/internal/db"
)

type demoCourse struct {
	title       string
	slug        string
	description string
}

var categoryTopics = map[string][]demoCourse{
	"dddddddd-1111-1111-1111-111111111111": { // Programming
		{"Go для начинающих", "go-for-beginners-demo", "Знакомство с языком Go: синтаксис, типы данных, горутины и работа с модулями."},
		{"Продвинутый Golang: конкурентность и каналы", "advanced-golang-concurrency-demo", "Глубокое погружение в goroutines, channels, sync и паттерны конкурентного Go."},
		{"Python: с нуля до промышленной разработки", "python-from-scratch-demo", "Основы Python, работа с пакетами, тестирование и подготовка к продакшену."},
		{"Java и Spring Boot: создание REST API", "java-spring-boot-rest-demo", "Разработка REST API на Java с использованием Spring Boot и Spring Data."},
		{"C# и .NET: разработка корпоративных приложений", "csharp-dotnet-enterprise-demo", "Построение корпоративных приложений на C# и платформе .NET."},
		{"Rust: системное программирование без страха", "rust-systems-programming-demo", "Владение памятью, безопасность типов и производительность в Rust."},
		{"Алгоритмы и структуры данных на практике", "algorithms-data-structures-demo", "Практический курс по алгоритмам, сложности и структурам данных."},
		{"Чистая архитектура и SOLID в реальных проектах", "clean-architecture-solid-demo", "Принципы SOLID и чистой архитектуры на примерах реальных проектов."},
		{"Тестирование кода: unit, integration, TDD", "testing-unit-integration-tdd-demo", "Модульное и интеграционное тестирование, разработка через тестирование."},
		{"Микросервисы на Go: gRPC и message queues", "go-microservices-grpc-demo", "Построение микросервисной архитектуры на Go с gRPC и очередями сообщений."},
	},
	"dddddddd-2222-2222-2222-222222222222": { // Databases
		{"PostgreSQL: от основ до оптимизации запросов", "postgresql-basics-optimization-demo", "Полный курс по PostgreSQL: от простых SELECT до оптимизации сложных запросов."},
		{"Проектирование реляционных баз данных", "relational-db-design-demo", "Нормализация, связи между таблицами и проектирование схем баз данных."},
		{"MongoDB для разработчиков: документные модели", "mongodb-document-models-demo", "Работа с документной моделью данных MongoDB и агрегациями."},
		{"Redis: кэширование и очереди в высоконагруженных системах", "redis-caching-queues-demo", "Использование Redis для кэширования, очередей и pub/sub в нагруженных системах."},
		{"SQL для аналитиков: сложные запросы и оконные функции", "sql-window-functions-demo", "Оконные функции, CTE и аналитические запросы на SQL."},
		{"Индексы и производительность PostgreSQL", "postgresql-indexes-performance-demo", "GIN, GiST, B-tree индексы и анализ производительности запросов PostgreSQL."},
		{"Репликация и отказоустойчивость баз данных", "db-replication-ha-demo", "Настройка репликации и построение отказоустойчивых баз данных."},
		{"ClickHouse: аналитика больших данных", "clickhouse-big-data-analytics-demo", "Аналитические запросы и хранение больших объёмов данных в ClickHouse."},
		{"Транзакции и уровни изоляции в PostgreSQL", "postgresql-transactions-isolation-demo", "ACID, уровни изоляции транзакций и блокировки в PostgreSQL."},
		{"Миграции схемы БД: практики безопасных изменений", "db-schema-migrations-demo", "Безопасные миграции схемы базы данных без простоя production."},
	},
	"dddddddd-3333-3333-3333-333333333333": { // DevOps
		{"Docker и контейнеризация приложений", "docker-containerization-demo", "Основы Docker: образы, контейнеры, Docker Compose для локальной разработки."},
		{"Kubernetes: оркестрация контейнеров в production", "kubernetes-orchestration-demo", "Развёртывание и управление контейнерами в Kubernetes кластере."},
		{"CI/CD с GitHub Actions: автоматизация деплоя", "cicd-github-actions-demo", "Построение пайплайнов CI/CD и автоматизация деплоя с GitHub Actions."},
		{"Terraform: инфраструктура как код", "terraform-infrastructure-as-code-demo", "Управление облачной инфраструктурой как кодом с помощью Terraform."},
		{"Nginx: настройка reverse proxy и балансировки", "nginx-reverse-proxy-demo", "Конфигурация Nginx как reverse proxy и балансировщика нагрузки."},
		{"Мониторинг и логирование: Prometheus и Grafana", "monitoring-prometheus-grafana-demo", "Сбор метрик и построение дашбордов с Prometheus и Grafana."},
		{"Ansible: автоматизация конфигурации серверов", "ansible-server-automation-demo", "Автоматизация настройки и конфигурации серверов с Ansible."},
		{"Linux администрирование для DevOps инженера", "linux-administration-devops-demo", "Практические навыки администрирования Linux серверов для DevOps."},
		{"Безопасность инфраструктуры и секреты в Vault", "infra-security-vault-secrets-demo", "Управление секретами и безопасность инфраструктуры с HashiCorp Vault."},
		{"AWS для начинающих: облачная инфраструктура", "aws-basics-cloud-demo", "Основы облачной инфраструктуры Amazon Web Services для новичков."},
	},
	"dddddddd-4444-4444-4444-444444444444": { // Frontend
		{"React с нуля: компоненты и хуки", "react-from-scratch-demo", "Компоненты, хуки и жизненный цикл в React с нуля."},
		{"TypeScript для React разработчиков", "typescript-for-react-demo", "Типизация React-приложений с помощью TypeScript."},
		{"Next.js: серверный рендеринг и App Router", "nextjs-app-router-demo", "Server Components, App Router и серверный рендеринг в Next.js."},
		{"Vue.js 3: composition API на практике", "vuejs3-composition-api-demo", "Практическое использование Composition API во Vue.js 3."},
		{"CSS Grid и Flexbox: современная вёрстка", "css-grid-flexbox-demo", "Современная адаптивная вёрстка с CSS Grid и Flexbox."},
		{"State management: Redux и Zustand", "state-management-redux-zustand-demo", "Управление состоянием приложения с Redux и Zustand."},
		{"GraphQL на клиенте: Apollo Client", "graphql-apollo-client-demo", "Работа с GraphQL API на клиенте с помощью Apollo Client."},
		{"Оптимизация производительности веб-приложений", "frontend-performance-optimization-demo", "Техники оптимизации производительности современных веб-приложений."},
		{"Доступность (a11y) в современных интерфейсах", "web-accessibility-a11y-demo", "Построение доступных интерфейсов и соответствие стандартам a11y."},
		{"Тестирование frontend: Jest и Testing Library", "frontend-testing-jest-demo", "Модульное и компонентное тестирование frontend с Jest и Testing Library."},
	},
}

var levels = []string{"beginner", "intermediate", "advanced"}
var accessTypes = []string{"free", "subscription"}

func main() {
	cfg := config.Load()
	ctx := context.Background()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer pool.Close()

	inserted := 0
	i := 0
	for categoryID, courses := range categoryTopics {
		for _, dc := range courses {
			level := levels[i%len(levels)]
			accessType := accessTypes[i%len(accessTypes)]
			i++

			tag, err := pool.Exec(ctx, `
				INSERT INTO courses (title, slug, description, level, access_type, published, category_id)
				VALUES ($1, $2, $3, $4, $5, true, $6)
				ON CONFLICT (slug) DO NOTHING
			`, dc.title, dc.slug, dc.description, level, accessType, categoryID)
			if err != nil {
				log.Fatalf("insert course %q: %v", dc.slug, err)
			}
			inserted += int(tag.RowsAffected())
		}
	}

	fmt.Printf("seed-demo-courses: inserted %d new course(s) (skipped duplicates already present)\n", inserted)
}
