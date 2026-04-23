package dbmate_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/amacneil/dbmate/v2/pkg/dbmate"
	"github.com/stretchr/testify/require"
)

func TestPrintDashboardTableEmpty(t *testing.T) {
	result := &dbmate.DashboardResult{
		TotalApplied: 0,
		TotalPending: 0,
		Total:        0,
		Migrations:   nil,
	}

	var buf bytes.Buffer
	dbmate.PrintDashboardTable(&buf, result, false, false)

	require.Equal(t, "No migrations found.\n", buf.String())
}

func TestPrintDashboardTableWithMigrations(t *testing.T) {
	result := &dbmate.DashboardResult{
		TotalApplied: 2,
		TotalPending: 1,
		Total:        3,
		Migrations: []dbmate.MigrationStatus{
			{
				Version:  "20230101000000",
				FileName: "20230101000000_create_users.sql",
				Applied:  true,
				Status:   "applied",
				Modified: "2023-01-01T00:00:00Z",
			},
			{
				Version:  "20230201000000",
				FileName: "20230201000000_create_posts.sql",
				Applied:  true,
				Status:   "applied",
				Modified: "2023-02-01T00:00:00Z",
			},
			{
				Version:  "20230301000000",
				FileName: "20230301000000_add_email.sql",
				Applied:  false,
				Status:   "pending",
			},
		},
	}

	var buf bytes.Buffer
	dbmate.PrintDashboardTable(&buf, result, false, false)

	output := buf.String()

	// Check table structure
	require.Contains(t, output, "Status")
	require.Contains(t, output, "Version")
	require.Contains(t, output, "Migration Name")
	require.Contains(t, output, "Modified")

	// Check status markers
	require.Contains(t, output, "[X]")
	require.Contains(t, output, "[ ]")

	// Check migration names
	require.Contains(t, output, "20230101000000_create_users.sql")
	require.Contains(t, output, "20230201000000_create_posts.sql")
	require.Contains(t, output, "20230301000000_add_email.sql")

	// Check summary
	require.Contains(t, output, "Applied: 2")
	require.Contains(t, output, "Pending: 1")
	require.Contains(t, output, "Total: 3")
}

func TestPrintDashboardTableAlignment(t *testing.T) {
	result := &dbmate.DashboardResult{
		TotalApplied: 1,
		TotalPending: 1,
		Total:        2,
		Migrations: []dbmate.MigrationStatus{
			{
				Version:  "1",
				FileName: "1_short.sql",
				Applied:  true,
				Status:   "applied",
			},
			{
				Version:  "20230101000000",
				FileName: "20230101000000_a_very_long_migration_name.sql",
				Applied:  false,
				Status:   "pending",
			},
		},
	}

	var buf bytes.Buffer
	dbmate.PrintDashboardTable(&buf, result, false, false)

	output := buf.String()

	// Verify separators exist (table structure)
	require.Contains(t, output, "+")
	require.Contains(t, output, "-")
	require.Contains(t, output, "|")

	// All separator lines should have the same length
	lines := strings.Split(output, "\n")
	var separatorLines []string
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "+") {
			separatorLines = append(separatorLines, l)
		}
	}
	if len(separatorLines) >= 2 {
		require.Equal(t, len(separatorLines[0]), len(separatorLines[1]),
			"separator lines should have consistent width")
	}
}

func TestPrintDashboardJSON(t *testing.T) {
	result := &dbmate.DashboardResult{
		TotalApplied: 2,
		TotalPending: 1,
		Total:        3,
		Migrations: []dbmate.MigrationStatus{
			{
				Version:  "20230101000000",
				FileName: "20230101000000_create_users.sql",
				Applied:  true,
				Status:   "applied",
				Modified: "2023-01-01T00:00:00Z",
			},
			{
				Version:  "20230201000000",
				FileName: "20230201000000_create_posts.sql",
				Applied:  true,
				Status:   "applied",
			},
			{
				Version:  "20230301000000",
				FileName: "20230301000000_add_email.sql",
				Applied:  false,
				Status:   "pending",
			},
		},
	}

	var buf bytes.Buffer
	err := dbmate.PrintDashboardJSON(&buf, result, "  ")
	require.NoError(t, err)

	// Verify it's valid JSON
	var parsed dbmate.DashboardResult
	err = json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)

	require.Equal(t, 2, parsed.TotalApplied)
	require.Equal(t, 1, parsed.TotalPending)
	require.Equal(t, 3, parsed.Total)
	require.Len(t, parsed.Migrations, 3)

	// Check first migration
	require.Equal(t, "20230101000000", parsed.Migrations[0].Version)
	require.Equal(t, "20230101000000_create_users.sql", parsed.Migrations[0].FileName)
	require.True(t, parsed.Migrations[0].Applied)
	require.Equal(t, "applied", parsed.Migrations[0].Status)
	require.Equal(t, "2023-01-01T00:00:00Z", parsed.Migrations[0].Modified)

	// Check pending migration
	require.Equal(t, "20230301000000", parsed.Migrations[2].Version)
	require.False(t, parsed.Migrations[2].Applied)
	require.Equal(t, "pending", parsed.Migrations[2].Status)
}

func TestPrintDashboardJSONEmpty(t *testing.T) {
	result := &dbmate.DashboardResult{
		TotalApplied: 0,
		TotalPending: 0,
		Total:        0,
		Migrations:   nil,
	}

	var buf bytes.Buffer
	err := dbmate.PrintDashboardJSON(&buf, result, "  ")
	require.NoError(t, err)

	var parsed dbmate.DashboardResult
	err = json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)

	require.Equal(t, 0, parsed.TotalApplied)
	require.Equal(t, 0, parsed.TotalPending)
	require.Equal(t, 0, parsed.Total)
	require.Nil(t, parsed.Migrations)
}

func TestDashboardResultMigrationStatus(t *testing.T) {
	// Test that the MigrationStatus fields are correctly set
	ms := dbmate.MigrationStatus{
		Version:  "20230101000000",
		FileName: "20230101000000_create_users.sql",
		Applied:  true,
		Status:   "applied",
		Modified: "2023-01-01T00:00:00Z",
	}

	require.Equal(t, "20230101000000", ms.Version)
	require.Equal(t, "20230101000000_create_users.sql", ms.FileName)
	require.True(t, ms.Applied)
	require.Equal(t, "applied", ms.Status)
	require.Equal(t, "2023-01-01T00:00:00Z", ms.Modified)

	// Test pending migration
	msPending := dbmate.MigrationStatus{
		Version:  "20230301000000",
		FileName: "20230301000000_add_email.sql",
		Applied:  false,
		Status:   "pending",
	}
	require.False(t, msPending.Applied)
	require.Equal(t, "pending", msPending.Status)
	require.Empty(t, msPending.Modified)
}

func TestPrintDashboardJSONFieldNames(t *testing.T) {
	result := &dbmate.DashboardResult{
		TotalApplied: 1,
		TotalPending: 0,
		Total:        1,
		Migrations: []dbmate.MigrationStatus{
			{
				Version:  "20230101000000",
				FileName: "20230101000000_create_users.sql",
				Applied:  true,
				Status:   "applied",
				Modified: "2023-01-01T00:00:00Z",
			},
		},
	}

	var buf bytes.Buffer
	err := dbmate.PrintDashboardJSON(&buf, result, "  ")
	require.NoError(t, err)

	// Verify JSON field names use camelCase
	raw := buf.String()
	require.Contains(t, raw, `"totalApplied"`)
	require.Contains(t, raw, `"totalPending"`)
	require.Contains(t, raw, `"fileName"`)
	require.Contains(t, raw, `"migrations"`)
}

func TestPrintDashboardJSONPretty(t *testing.T) {
	result := &dbmate.DashboardResult{
		TotalApplied: 1,
		TotalPending: 0,
		Total:        1,
		Migrations: []dbmate.MigrationStatus{
			{Version: "1", FileName: "m1.sql", Applied: true, Status: "applied"},
		},
	}

	t.Run("indent with 2 spaces", func(t *testing.T) {
		var buf bytes.Buffer
		err := dbmate.PrintDashboardJSON(&buf, result, "  ")
		require.NoError(t, err)
		output := buf.String()
		require.Contains(t, output, "  ")
		require.Contains(t, output, "\n")
	})

	t.Run("indent with 4 spaces", func(t *testing.T) {
		var buf bytes.Buffer
		err := dbmate.PrintDashboardJSON(&buf, result, "    ")
		require.NoError(t, err)
		output := buf.String()
		require.Contains(t, output, "    ")
	})

	t.Run("empty indent is compact", func(t *testing.T) {
		var buf bytes.Buffer
		err := dbmate.PrintDashboardJSON(&buf, result, "")
		require.NoError(t, err)
		output := buf.String()
		// Compact JSON should be a single line
		lines := strings.Split(strings.TrimSpace(output), "\n")
		require.Equal(t, 1, len(lines), "compact JSON should be a single line")
	})
}

func TestPrintDashboardTableAllApplied(t *testing.T) {
	result := &dbmate.DashboardResult{
		TotalApplied: 2,
		TotalPending: 0,
		Total:        2,
		Migrations: []dbmate.MigrationStatus{
			{
				Version:  "20230101000000",
				FileName: "20230101000000_create_users.sql",
				Applied:  true,
				Status:   "applied",
			},
			{
				Version:  "20230201000000",
				FileName: "20230201000000_create_posts.sql",
				Applied:  true,
				Status:   "applied",
			},
		},
	}

	var buf bytes.Buffer
	dbmate.PrintDashboardTable(&buf, result, false, false)

	output := buf.String()
	require.Contains(t, output, "Applied: 2")
	require.Contains(t, output, "Pending: 0")
	require.Contains(t, output, "Total: 2")
	// Should not have [ ] marker
	require.NotContains(t, output, "[ ]")
}

func TestPrintDashboardTableAllPending(t *testing.T) {
	result := &dbmate.DashboardResult{
		TotalApplied: 0,
		TotalPending: 2,
		Total:        2,
		Migrations: []dbmate.MigrationStatus{
			{
				Version:  "20230101000000",
				FileName: "20230101000000_create_users.sql",
				Applied:  false,
				Status:   "pending",
			},
			{
				Version:  "20230201000000",
				FileName: "20230201000000_create_posts.sql",
				Applied:  false,
				Status:   "pending",
			},
		},
	}

	var buf bytes.Buffer
	dbmate.PrintDashboardTable(&buf, result, false, false)

	output := buf.String()
	require.Contains(t, output, "Applied: 0")
	require.Contains(t, output, "Pending: 2")
	require.Contains(t, output, "Total: 2")
	// Should not have [X] marker
	require.NotContains(t, output, "[X]")
}

func TestPrintDashboardTableNoHeader(t *testing.T) {
	result := &dbmate.DashboardResult{
		TotalApplied: 1,
		TotalPending: 1,
		Total:        2,
		Migrations: []dbmate.MigrationStatus{
			{
				Version:  "20230101000000",
				FileName: "20230101000000_create_users.sql",
				Applied:  true,
				Status:   "applied",
				Modified: "2023-01-01T00:00:00Z",
			},
			{
				Version:  "20230201000000",
				FileName: "20230201000000_create_posts.sql",
				Applied:  false,
				Status:   "pending",
			},
		},
	}

	// With header (default)
	var bufWithHeader bytes.Buffer
	dbmate.PrintDashboardTable(&bufWithHeader, result, false, false)
	outputWithHeader := bufWithHeader.String()
	require.Contains(t, outputWithHeader, "| Status ")
	require.Contains(t, outputWithHeader, "| Version ")
	require.Contains(t, outputWithHeader, "| Migration Name ")
	require.Contains(t, outputWithHeader, "| Modified ")

	// Without header (--no-header)
	var bufNoHeader bytes.Buffer
	dbmate.PrintDashboardTable(&bufNoHeader, result, true, false)
	outputNoHeader := bufNoHeader.String()
	require.NotContains(t, outputNoHeader, "| Status ")
	require.NotContains(t, outputNoHeader, "| Version ")
	require.NotContains(t, outputNoHeader, "| Migration Name ")
	require.NotContains(t, outputNoHeader, "| Modified ")

	// Both should have data rows and summary
	for _, output := range []string{outputWithHeader, outputNoHeader} {
		require.Contains(t, output, "[X]")
		require.Contains(t, output, "[ ]")
		require.Contains(t, output, "20230101000000_create_users.sql")
		require.Contains(t, output, "20230201000000_create_posts.sql")
		require.Contains(t, output, "Applied: 1")
		require.Contains(t, output, "Pending: 1")
		require.Contains(t, output, "Total: 2")
	}
}

func TestSortDashboardResult(t *testing.T) {
	result := &dbmate.DashboardResult{
		TotalApplied: 2,
		TotalPending: 2,
		Total:        4,
		Migrations: []dbmate.MigrationStatus{
			{
				Version:  "20230201000000",
				FileName: "20230201000000_create_posts.sql",
				Applied:  true,
				Status:   "applied",
				Modified: "2023-02-01T00:00:00Z",
			},
			{
				Version:  "20230401000000",
				FileName: "20230401000000_add_tags.sql",
				Applied:  false,
				Status:   "pending",
			},
			{
				Version:  "20230101000000",
				FileName: "20230101000000_create_users.sql",
				Applied:  true,
				Status:   "applied",
				Modified: "2023-01-01T00:00:00Z",
			},
			{
				Version:  "20230301000000",
				FileName: "20230301000000_add_email.sql",
				Applied:  false,
				Status:   "pending",
			},
		},
	}

	t.Run("sort by version (default)", func(t *testing.T) {
		r := cloneDashboardResult(result)
		dbmate.SortDashboardResult(r, "version")
		require.Equal(t, "20230101000000", r.Migrations[0].Version)
		require.Equal(t, "20230201000000", r.Migrations[1].Version)
		require.Equal(t, "20230301000000", r.Migrations[2].Version)
		require.Equal(t, "20230401000000", r.Migrations[3].Version)
	})

	t.Run("sort by status (applied first)", func(t *testing.T) {
		r := cloneDashboardResult(result)
		dbmate.SortDashboardResult(r, "status")
		// applied first
		require.True(t, r.Migrations[0].Applied)
		require.True(t, r.Migrations[1].Applied)
		require.False(t, r.Migrations[2].Applied)
		require.False(t, r.Migrations[3].Applied)
		// within same status, sorted by version
		require.Equal(t, "20230101000000", r.Migrations[0].Version)
		require.Equal(t, "20230201000000", r.Migrations[1].Version)
		require.Equal(t, "20230301000000", r.Migrations[2].Version)
		require.Equal(t, "20230401000000", r.Migrations[3].Version)
	})

	t.Run("sort by name", func(t *testing.T) {
		r := cloneDashboardResult(result)
		dbmate.SortDashboardResult(r, "name")
		require.Equal(t, "20230101000000_create_users.sql", r.Migrations[0].FileName)
		require.Equal(t, "20230201000000_create_posts.sql", r.Migrations[1].FileName)
		require.Equal(t, "20230301000000_add_email.sql", r.Migrations[2].FileName)
		require.Equal(t, "20230401000000_add_tags.sql", r.Migrations[3].FileName)
	})

	t.Run("sort by modified (most recent first)", func(t *testing.T) {
		r := cloneDashboardResult(result)
		dbmate.SortDashboardResult(r, "modified")
		// entries with modified dates come first (most recent), then empty
		require.Equal(t, "2023-02-01T00:00:00Z", r.Migrations[0].Modified)
		require.Equal(t, "2023-01-01T00:00:00Z", r.Migrations[1].Modified)
		// entries without modified dates come after, sorted by version
		require.Equal(t, "20230301000000", r.Migrations[2].Version)
		require.Equal(t, "20230401000000", r.Migrations[3].Version)
	})

	t.Run("sort by empty string defaults to version", func(t *testing.T) {
		r := cloneDashboardResult(result)
		dbmate.SortDashboardResult(r, "")
		require.Equal(t, "20230101000000", r.Migrations[0].Version)
	})
}

// cloneDashboardResult creates a deep copy for test isolation
func cloneDashboardResult(r *dbmate.DashboardResult) *dbmate.DashboardResult {
	clone := &dbmate.DashboardResult{
		TotalApplied: r.TotalApplied,
		TotalPending: r.TotalPending,
		Total:        r.Total,
		Migrations:   make([]dbmate.MigrationStatus, len(r.Migrations)),
	}
	copy(clone.Migrations, r.Migrations)
	return clone
}

func TestFilterDashboardResult(t *testing.T) {
	result := &dbmate.DashboardResult{
		TotalApplied: 2,
		TotalPending: 3,
		Total:        5,
		Migrations: []dbmate.MigrationStatus{
			{Version: "1", FileName: "m1.sql", Applied: true, Status: "applied"},
			{Version: "2", FileName: "m2.sql", Applied: true, Status: "applied"},
			{Version: "3", FileName: "m3.sql", Applied: false, Status: "pending"},
			{Version: "4", FileName: "m4.sql", Applied: false, Status: "pending"},
			{Version: "5", FileName: "m5.sql", Applied: false, Status: "pending"},
		},
	}

	t.Run("no filter when empty", func(t *testing.T) {
		r := cloneDashboardResult(result)
		dbmate.FilterDashboardResult(r, "")
		require.Len(t, r.Migrations, 5)
		require.Equal(t, 2, r.TotalApplied)
		require.Equal(t, 3, r.TotalPending)
		require.Equal(t, 5, r.Total)
	})

	t.Run("filter applied", func(t *testing.T) {
		r := cloneDashboardResult(result)
		dbmate.FilterDashboardResult(r, "applied")
		require.Len(t, r.Migrations, 2)
		require.Equal(t, "1", r.Migrations[0].Version)
		require.Equal(t, "2", r.Migrations[1].Version)
		require.Equal(t, 2, r.TotalApplied)
		require.Equal(t, 0, r.TotalPending)
		require.Equal(t, 2, r.Total)
	})

	t.Run("filter pending", func(t *testing.T) {
		r := cloneDashboardResult(result)
		dbmate.FilterDashboardResult(r, "pending")
		require.Len(t, r.Migrations, 3)
		require.Equal(t, "3", r.Migrations[0].Version)
		require.Equal(t, "4", r.Migrations[1].Version)
		require.Equal(t, "5", r.Migrations[2].Version)
		require.Equal(t, 0, r.TotalApplied)
		require.Equal(t, 3, r.TotalPending)
		require.Equal(t, 3, r.Total)
	})

	t.Run("filter with no matches", func(t *testing.T) {
		r := &dbmate.DashboardResult{
			TotalApplied: 2,
			TotalPending: 0,
			Total:        2,
			Migrations: []dbmate.MigrationStatus{
				{Version: "1", FileName: "m1.sql", Applied: true, Status: "applied"},
				{Version: "2", FileName: "m2.sql", Applied: true, Status: "applied"},
			},
		}
		dbmate.FilterDashboardResult(r, "pending")
		require.Len(t, r.Migrations, 0)
		require.Equal(t, 0, r.TotalApplied)
		require.Equal(t, 0, r.TotalPending)
		require.Equal(t, 0, r.Total)
	})
}

func TestLimitDashboardResult(t *testing.T) {
	result := &dbmate.DashboardResult{
		TotalApplied: 2,
		TotalPending: 3,
		Total:        5,
		Migrations: []dbmate.MigrationStatus{
			{Version: "1", FileName: "m1.sql", Applied: true, Status: "applied"},
			{Version: "2", FileName: "m2.sql", Applied: true, Status: "applied"},
			{Version: "3", FileName: "m3.sql", Applied: false, Status: "pending"},
			{Version: "4", FileName: "m4.sql", Applied: false, Status: "pending"},
			{Version: "5", FileName: "m5.sql", Applied: false, Status: "pending"},
		},
	}

	t.Run("no limit when limit=0", func(t *testing.T) {
		r := cloneDashboardResult(result)
		dbmate.LimitDashboardResult(r, 0)
		require.Len(t, r.Migrations, 5)
	})

	t.Run("no limit when limit negative", func(t *testing.T) {
		r := cloneDashboardResult(result)
		dbmate.LimitDashboardResult(r, -1)
		require.Len(t, r.Migrations, 5)
	})

	t.Run("limit to 2", func(t *testing.T) {
		r := cloneDashboardResult(result)
		dbmate.LimitDashboardResult(r, 2)
		require.Len(t, r.Migrations, 2)
		require.Equal(t, "1", r.Migrations[0].Version)
		require.Equal(t, "2", r.Migrations[1].Version)
	})

	t.Run("limit larger than total", func(t *testing.T) {
		r := cloneDashboardResult(result)
		dbmate.LimitDashboardResult(r, 100)
		require.Len(t, r.Migrations, 5)
	})

	t.Run("limit equal to total", func(t *testing.T) {
		r := cloneDashboardResult(result)
		dbmate.LimitDashboardResult(r, 5)
		require.Len(t, r.Migrations, 5)
	})
}

func TestPaginateDashboardResult(t *testing.T) {
	result := &dbmate.DashboardResult{
		TotalApplied: 2,
		TotalPending: 3,
		Total:        5,
		Migrations: []dbmate.MigrationStatus{
			{Version: "1", FileName: "m1.sql", Applied: true, Status: "applied"},
			{Version: "2", FileName: "m2.sql", Applied: true, Status: "applied"},
			{Version: "3", FileName: "m3.sql", Applied: false, Status: "pending"},
			{Version: "4", FileName: "m4.sql", Applied: false, Status: "pending"},
			{Version: "5", FileName: "m5.sql", Applied: false, Status: "pending"},
		},
	}

	t.Run("no pagination when page=0", func(t *testing.T) {
		r := cloneDashboardResult(result)
		dbmate.PaginateDashboardResult(r, 0, 2)
		require.Len(t, r.Migrations, 5)
		require.Equal(t, 0, r.Page)
		require.Equal(t, 0, r.PageSize)
		require.Equal(t, 0, r.TotalPages)
	})

	t.Run("no pagination when pageSize=0", func(t *testing.T) {
		r := cloneDashboardResult(result)
		dbmate.PaginateDashboardResult(r, 1, 0)
		require.Len(t, r.Migrations, 5)
		require.Equal(t, 0, r.Page)
	})

	t.Run("page 1 with size 2", func(t *testing.T) {
		r := cloneDashboardResult(result)
		dbmate.PaginateDashboardResult(r, 1, 2)
		require.Len(t, r.Migrations, 2)
		require.Equal(t, "1", r.Migrations[0].Version)
		require.Equal(t, "2", r.Migrations[1].Version)
		require.Equal(t, 1, r.Page)
		require.Equal(t, 2, r.PageSize)
		require.Equal(t, 3, r.TotalPages)
	})

	t.Run("page 2 with size 2", func(t *testing.T) {
		r := cloneDashboardResult(result)
		dbmate.PaginateDashboardResult(r, 2, 2)
		require.Len(t, r.Migrations, 2)
		require.Equal(t, "3", r.Migrations[0].Version)
		require.Equal(t, "4", r.Migrations[1].Version)
		require.Equal(t, 2, r.Page)
		require.Equal(t, 3, r.TotalPages)
	})

	t.Run("last page with remaining items", func(t *testing.T) {
		r := cloneDashboardResult(result)
		dbmate.PaginateDashboardResult(r, 3, 2)
		require.Len(t, r.Migrations, 1)
		require.Equal(t, "5", r.Migrations[0].Version)
		require.Equal(t, 3, r.Page)
		require.Equal(t, 3, r.TotalPages)
	})

	t.Run("page exceeds total pages is clamped", func(t *testing.T) {
		r := cloneDashboardResult(result)
		dbmate.PaginateDashboardResult(r, 10, 2)
		require.Len(t, r.Migrations, 1)
		require.Equal(t, "5", r.Migrations[0].Version)
		require.Equal(t, 3, r.Page) // clamped to last page
	})

	t.Run("page size larger than total", func(t *testing.T) {
		r := cloneDashboardResult(result)
		dbmate.PaginateDashboardResult(r, 1, 100)
		require.Len(t, r.Migrations, 5)
		require.Equal(t, 1, r.Page)
		require.Equal(t, 1, r.TotalPages)
	})

	t.Run("empty migrations", func(t *testing.T) {
		r := &dbmate.DashboardResult{
			TotalApplied: 0,
			TotalPending: 0,
			Total:        0,
			Migrations:   nil,
		}
		dbmate.PaginateDashboardResult(r, 1, 10)
		require.Len(t, r.Migrations, 0)
		require.Equal(t, 1, r.Page)
		require.Equal(t, 1, r.TotalPages)
	})
}

func TestPrintDashboardTablePagination(t *testing.T) {
	result := &dbmate.DashboardResult{
		TotalApplied: 2,
		TotalPending: 1,
		Total:        3,
		Page:         1,
		PageSize:     2,
		TotalPages:   2,
		Migrations: []dbmate.MigrationStatus{
			{Version: "20230101000000", FileName: "20230101000000_create_users.sql", Applied: true, Status: "applied"},
			{Version: "20230201000000", FileName: "20230201000000_create_posts.sql", Applied: true, Status: "applied"},
		},
	}

	var buf bytes.Buffer
	dbmate.PrintDashboardTable(&buf, result, false, false)

	output := buf.String()
	require.Contains(t, output, "Applied: 2")
	require.Contains(t, output, "Pending: 1")
	require.Contains(t, output, "Total: 3")
	require.Contains(t, output, "Page: 1/2")
	require.Contains(t, output, "Page Size: 2")
}

func TestPrintDashboardTableNoPagination(t *testing.T) {
	result := &dbmate.DashboardResult{
		TotalApplied: 1,
		TotalPending: 0,
		Total:        1,
		Page:         0,
		PageSize:     0,
		TotalPages:   0,
		Migrations: []dbmate.MigrationStatus{
			{Version: "20230101000000", FileName: "20230101000000_create_users.sql", Applied: true, Status: "applied"},
		},
	}

	var buf bytes.Buffer
	dbmate.PrintDashboardTable(&buf, result, false, false)

	output := buf.String()
	require.NotContains(t, output, "Page:")
	require.NotContains(t, output, "Page Size:")
}

func TestPrintDashboardTableColor(t *testing.T) {
	result := &dbmate.DashboardResult{
		TotalApplied: 1,
		TotalPending: 1,
		Total:        2,
		Migrations: []dbmate.MigrationStatus{
			{Version: "20230101000000", FileName: "20230101000000_create_users.sql", Applied: true, Status: "applied"},
			{Version: "20230201000000", FileName: "20230201000000_add_email.sql", Applied: false, Status: "pending"},
		},
	}

	t.Run("with color", func(t *testing.T) {
		var buf bytes.Buffer
		dbmate.PrintDashboardTable(&buf, result, false, true)
		output := buf.String()
		// ANSI escape codes should be present
		require.Contains(t, output, "\033[")
		require.Contains(t, output, "\033[32m") // green for applied
		require.Contains(t, output, "\033[33m") // yellow for pending
		require.Contains(t, output, "\033[0m")  // reset
	})

	t.Run("without color", func(t *testing.T) {
		var buf bytes.Buffer
		dbmate.PrintDashboardTable(&buf, result, false, false)
		output := buf.String()
		require.NotContains(t, output, "\033[")
	})

	t.Run("color summary", func(t *testing.T) {
		var buf bytes.Buffer
		dbmate.PrintDashboardTable(&buf, result, false, true)
		output := buf.String()
		require.Contains(t, output, "\033[32mApplied:")
		require.Contains(t, output, "\033[33mPending:")
		require.Contains(t, output, "\033[36mTotal:")
	})
}
