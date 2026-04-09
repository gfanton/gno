package mcp

import "testing"

func TestDescriptions_AllSections(t *testing.T) {
	expected := []string{
		"instructions",
		"node_overview", "block_inspect", "peer_consensus",
		"logs_search", "logs_summary", "logs_navigate",
		"node_data_open", "node_data_block", "node_data_tx", "node_data_wal", "node_data_state",
		"node_compare",
		"realm_eval", "realm_inspect", "realm_source",
		"account_info", "genesis_info",
		"chain_query",
		"node_doctor",
	}
	for _, name := range expected {
		if _, ok := descriptions[name]; !ok {
			t.Errorf("missing description section: %q", name)
		}
	}
}

func TestDescriptions_NonEmpty(t *testing.T) {
	for name, body := range descriptions {
		if body == "" {
			t.Errorf("section %q has empty body", name)
		}
	}
}
