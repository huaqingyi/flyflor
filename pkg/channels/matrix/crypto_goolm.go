//go:build goolm

package matrix

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"go.mau.fi/util/dbutil"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	_ "modernc.org/sqlite"

	"github.com/sipeed/picoclaw/pkg/logger"
)

func (c *MatrixChannel) initCrypto(ctx context.Context) error {
	logger.InfoC("matrix", "Initializing crypto helper")

	if err := os.MkdirAll(c.cryptoDbPath, 0o700); err != nil {
		return fmt.Errorf("create crypto database directory: %w", err)
	}

	dbPath := filepath.Join(c.cryptoDbPath, dbName)
	connStr := "file:" + dbPath + "?_foreign_keys=on"

	db, err := sql.Open(sqliteDriver, connStr)
	if err != nil {
		return fmt.Errorf("open crypto database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	pragmaStmts := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA busy_timeout = 5000",
	}
	for _, pragma := range pragmaStmts {
		if _, err = db.ExecContext(ctx, pragma); err != nil {
			_ = db.Close()
			return fmt.Errorf("execute %s: %w", pragma, err)
		}
	}

	wrappedDB, err := dbutil.NewWithDB(db, sqliteDriver)
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("wrap database: %w", err)
	}

	cryptoHelper, err := cryptohelper.NewCryptoHelper(c.client, []byte(c.config.CryptoPassphrase), wrappedDB)
	if err != nil {
		return fmt.Errorf("create crypto helper: %w", err)
	}

	if c.client.DeviceID == "" {
		resp, whoamiErr := c.client.Whoami(ctx)
		if whoamiErr != nil {
			_ = db.Close()
			return fmt.Errorf("get device ID via whoami: %w", whoamiErr)
		}
		c.client.DeviceID = resp.DeviceID
	}

	if err = cryptoHelper.Init(ctx); err != nil {
		_ = cryptoHelper.Close()
		return fmt.Errorf("init crypto helper: %w", err)
	}

	c.client.Crypto = cryptoHelper
	c.cryptoHelper = cryptoHelper

	logger.InfoC("matrix", "Crypto helper initialized successfully")
	return nil
}
