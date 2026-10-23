package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseCommandsJSONCatalog runs `cmc commands -o json` and unmarshals the catalog.
func parseCommandsJSONCatalog(t *testing.T) commandCatalog {
	t.Helper()
	stdout, _, err := executeCommand(t, "commands", "-o", "json")
	require.NoError(t, err)
	var cat commandCatalog
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(stdout)), &cat))
	return cat
}

func TestCommands_JSONCatalog_HasTopLevelFields(t *testing.T) {
	cat := parseCommandsJSONCatalog(t)

	require.NotEmpty(t, cat.Version, "catalog.version must be non-empty")
	require.Equal(t, version, cat.Version, "catalog.version should match embedded version var")

	require.NotEmpty(t, cat.OASRepo, "catalog.oas_repo must be non-empty")
	require.Equal(t, oasRepo, cat.OASRepo)

	require.NotEmpty(t, cat.Commands, "catalog.commands must be non-empty")
	// Completeness: every commandMeta entry is emitted, plus auth/status/version and
	// tui subcommands (no metadata map entry). A stub catalog with one command fails here.
	const nonMetaCatalogCommands = 5 // auth, status, version, tui markets, tui trending
	require.GreaterOrEqual(t, len(cat.Commands), len(commandMeta)+nonMetaCatalogCommands,
		"catalog must list all documented API commands plus auth, status, version, and tui subcommands")
}

func TestCommands_JSONCatalog_RepresentativeCommandsAndUniqueNames(t *testing.T) {
	cat := parseCommandsJSONCatalog(t)

	names := make(map[string]struct{}, len(cat.Commands))
	for _, c := range cat.Commands {
		require.NotEmpty(t, c.Name, "catalog entry must have non-empty name")
		_, dup := names[c.Name]
		require.False(t, dup, "catalog command names must be unique; duplicate %q", c.Name)
		names[c.Name] = struct{}{}
	}

	for _, want := range []string{
		"metrics",
		"news",
		"pairs",
		"price",
		"markets",
		"history",
		"resolve",
		"version",
		"search",
		"top-gainers-losers",
		"tui markets",
		"tui trending",
	} {
		_, ok := names[want]
		require.True(t, ok, "catalog must include expected command %q", want)
	}
}

func TestCommands_JSONCatalog_CommandMetaKeysAppearInCatalog(t *testing.T) {
	cat := parseCommandsJSONCatalog(t)

	names := make(map[string]struct{}, len(cat.Commands))
	for _, c := range cat.Commands {
		names[c.Name] = struct{}{}
	}

	for metaName := range commandMeta {
		_, ok := names[metaName]
		require.True(t, ok, "commandMeta key %q must appear in emitted catalog (orphaned metadata otherwise)", metaName)
	}
}

func TestCommands_ResolveCatalog_UsesSplitEndpoints(t *testing.T) {
	cat := parseCommandsJSONCatalog(t)

	var resolveEntry *commandInfo
	for i := range cat.Commands {
		if cat.Commands[i].Name == "resolve" {
			resolveEntry = &cat.Commands[i]
			break
		}
	}
	require.NotNil(t, resolveEntry, "catalog must include resolve entry")

	require.NotEmpty(t, resolveEntry.APIEndpoints, "resolve must advertise per-flag API endpoints")
	require.Equal(t, "/v2/cryptocurrency/info", resolveEntry.APIEndpoints["id"])
	require.Equal(t, "/v2/cryptocurrency/info", resolveEntry.APIEndpoints["slug"])
	require.Equal(t, "/v1/cryptocurrency/map", resolveEntry.APIEndpoints["symbol"])
	require.NotEmpty(t, resolveEntry.OASOperationIDs)
	require.Equal(t, "getV2CryptocurrencyInfo", resolveEntry.OASOperationIDs["id"])
	require.Equal(t, "getV2CryptocurrencyInfo", resolveEntry.OASOperationIDs["slug"])
	require.Equal(t, "getV1CryptocurrencyMap", resolveEntry.OASOperationIDs["symbol"])
}

func TestCommands_SearchCatalog_UsesAddressEndpoints(t *testing.T) {
	cat := parseCommandsJSONCatalog(t)

	var searchEntry *commandInfo
	for i := range cat.Commands {
		if cat.Commands[i].Name == "search" {
			searchEntry = &cat.Commands[i]
			break
		}
	}
	require.NotNil(t, searchEntry, "catalog must include search entry")
	require.NotEmpty(t, searchEntry.APIEndpoints, "search must advertise API endpoints")
	assert.Equal(t, "/v1/cryptocurrency/map", searchEntry.APIEndpoints["asset"])
	assert.Equal(t, "/v4/dex/pairs/quotes/latest", searchEntry.APIEndpoints["address_pair"])
	assert.Equal(t, "/v4/dex/spot-pairs/latest", searchEntry.APIEndpoints["address_token"])
	_, found := searchEntry.APIEndpoints["address_network"]
	assert.False(t, found, "search catalog must no longer advertise network-list endpoint")
}

func TestCommands_PriceCatalog_AdvertisesWithInfo(t *testing.T) {
	cat := parseCommandsJSONCatalog(t)

	priceEntry := commandInfoByName(t, cat, "price")
	require.NotEmpty(t, priceEntry.APIEndpoints, "price must advertise split endpoints")
	assert.Equal(t, "/v2/cryptocurrency/info", priceEntry.APIEndpoints["with-info"])
	assert.Equal(t, "/v1/blockchain/statistics/latest", priceEntry.APIEndpoints["with-chain-stats"])
	require.NotEmpty(t, priceEntry.OASOperationIDs, "price must advertise split OAS operation IDs")
	assert.Equal(t, "getV2CryptocurrencyInfo", priceEntry.OASOperationIDs["with-info"])
	assert.Equal(t, "getV1BlockchainStatisticsLatest", priceEntry.OASOperationIDs["with-chain-stats"])
}

func TestCommands_NewPublicCommandsAreCatalogued(t *testing.T) {
	cat := parseCommandsJSONCatalog(t)

	for _, name := range []string{"metrics", "news", "pairs"} {
		entry := commandInfoByName(t, cat, name)
		require.NotEmpty(t, entry.APIEndpoint, "%q must advertise an API endpoint", name)
		require.NotEmpty(t, entry.OASOperationID, "%q must advertise an OAS operation id", name)
	}
}

func TestCommands_CatalogExcludesHiddenAndShellCommands(t *testing.T) {
	cat := parseCommandsJSONCatalog(t)

	names := make(map[string]struct{}, len(cat.Commands))
	for _, c := range cat.Commands {
		names[c.Name] = struct{}{}
	}

	// Hidden catalog command itself must not appear; Cobra help/completion are skipped in runCommands.
	for _, excluded := range []string{"commands", "help", "completion"} {
		_, found := names[excluded]
		require.False(t, found, "catalog must not list excluded command %q", excluded)
	}
}

func TestCommands_CatalogListsDEXSubcommandsOnly(t *testing.T) {
	cat := parseCommandsJSONCatalog(t)

	for _, c := range cat.Commands {
		require.NotEqual(t, "dex", c.Name, "catalog lists dex subcommands, not the dex group itself")
	}
	assert.Equal(t, "/v4/dex/spot-pairs/latest", commandInfoByName(t, cat, "dex pairs").APIEndpoint)
	assert.Equal(t, "/v4/dex/pairs/quotes/latest", commandInfoByName(t, cat, "dex pair").APIEndpoint)
}

func TestCommands_VersionEntry_OutputFormats_TableThenJSON(t *testing.T) {
	cat := parseCommandsJSONCatalog(t)

	var versionEntry *commandInfo
	for i := range cat.Commands {
		if cat.Commands[i].Name == "version" {
			versionEntry = &cat.Commands[i]
			break
		}
	}
	require.NotNil(t, versionEntry, "catalog must include a version entry")

	require.Equal(t, []string{"table", "json"}, versionEntry.OutputFormats,
		"version entry must advertise table before json in output_formats")
}

func TestCommands_JSONCatalog_SpecialOutputFormatsContracts(t *testing.T) {
	cat := parseCommandsJSONCatalog(t)

	byName := make(map[string]commandInfo, len(cat.Commands))
	for _, c := range cat.Commands {
		byName[c.Name] = c
	}

	for _, name := range []string{"tui markets", "tui trending"} {
		c, ok := byName[name]
		require.True(t, ok, "catalog must include %q for output_formats contract", name)
		require.Equal(t, []string{"tui"}, c.OutputFormats,
			"%q must advertise output_formats exactly [tui]", name)
	}
}
