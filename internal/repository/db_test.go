//go:build integration

package repository_test

import (
	"context"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"
	testcontainers "github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/fairyhunter13/community-waste-collection-system/internal/repository"
)

type baseRepoSuite struct {
	suite.Suite
	db        *sqlx.DB
	container *tcpostgres.PostgresContainer
}

func (s *baseRepoSuite) SetupSuite() {
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("test_waste"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	s.Require().NoError(err)
	s.container = container

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	s.Require().NoError(err)

	s.db = repository.MustConnect(connStr)

	m, err := migrate.New("file://../../migrations", connStr)
	s.Require().NoError(err)
	s.Require().NoError(m.Up())
}

func (s *baseRepoSuite) TearDownTest() {
	_, err := s.db.ExecContext(context.Background(),
		`TRUNCATE households, waste_pickups, payments CASCADE`,
	)
	s.Require().NoError(err)
}

func (s *baseRepoSuite) TearDownSuite() {
	if s.db != nil {
		s.db.Close()
	}
	if s.container != nil {
		_ = s.container.Terminate(context.Background())
	}
}
