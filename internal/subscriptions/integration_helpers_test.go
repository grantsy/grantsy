package subscriptions_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/grantsy/grantsy/internal/subscriptions"
)

type dbFactory struct {
	name  string
	newDB func(t *testing.T) *subscriptions.Repo
}

var drivers []dbFactory

func TestMain(m *testing.M) {
	ctx := context.Background()

	// Start PostgreSQL container.
	pgConnStr, pgContainer, err := startPostgresContainer(ctx)
	if err != nil {
		log.Fatalf("failed to start postgres container: %v", err)
	}

	// Register drivers.
	drivers = append(drivers, dbFactory{
		name:  "sqlite",
		newDB: newSQLiteRepo,
	})
	drivers = append(drivers, dbFactory{
		name:  "postgres",
		newDB: newPostgresRepo(pgConnStr),
	})

	code := m.Run()

	if pgContainer != nil {
		if err := pgContainer.Terminate(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "failed to terminate postgres container: %v\n", err)
		}
	}

	os.Exit(code)
}

func testSub(id int, userID string, status string) *subscriptions.Subscription {
	now := time.Now().Unix()
	return &subscriptions.Subscription{
		ID:                 id,
		UserID:             userID,
		CustomerID:         1000 + id,
		OrderID:            2000 + id,
		ProductID:          12345,
		ProductName:        "Pro Plan",
		VariantID:          100,
		VariantName:        "Monthly",
		Status:             status,
		StatusFormatted:    status,
		CardBrand:          "visa",
		CardLastFour:       "4242",
		Cancelled:          false,
		TrialEndsAt:        nil,
		BillingAnchor:      1,
		SubscriptionItemID: 3000 + id,
		RenewsAt:           now + 86400*30,
		EndsAt:             nil,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}
