// MIT License
//
// Copyright (c) 2026 Kolin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
package database

import (
    "loglynx/internal/database/indexes"

    "github.com/pterm/pterm"
    "gorm.io/gorm"
)

// OptimizeDatabase applies additional optimizations after initial migrations.
// It reconciles expected performance indexes and verifies SQLite settings.
func OptimizeDatabase(db *gorm.DB, logger *pterm.Logger) error {
    logger.Debug("Applying database optimizations...")

    // Verify WAL mode is enabled (debug level - only show if there's a problem)
    var journalMode string
    if err := db.Raw("PRAGMA journal_mode").Scan(&journalMode).Error; err != nil {
        logger.Warn("Failed to check journal mode", logger.Args("error", err))
    } else if journalMode != "wal" {
        logger.Warn("Database not in WAL mode", logger.Args("mode", journalMode))
    } else {
        logger.Trace("Database journal mode verified", logger.Args("mode", journalMode))
    }

    // Verify page size (trace level - not critical)
    var pageSize int
    if err := db.Raw("PRAGMA page_size").Scan(&pageSize).Error; err != nil {
        logger.Debug("Failed to check page size", logger.Args("error", err))
    } else {
        logger.Trace("Database page size", logger.Args("bytes", pageSize))
    }

    created, dropped, err := indexes.Ensure(db, logger)
    if err != nil {
        return err
    }
    logger.Debug("Performance indexes reconciled", logger.Args("created", created, "dropped", dropped))

    // Analyze tables for query optimizer (only log if it fails)
    if err := db.Exec("ANALYZE").Error; err != nil {
        logger.Warn("Failed to analyze database", logger.Args("error", err))
    } else {
        logger.Trace("Database statistics analyzed")
    }

    // Hint SQLite to optimize query plans using collected stats
    if err := db.Exec("PRAGMA optimize").Error; err != nil {
        logger.Debug("PRAGMA optimize failed", logger.Args("error", err))
    } else {
        logger.Trace("PRAGMA optimize executed")
    }

    logger.Debug("Database optimizations completed")
    return nil
}


