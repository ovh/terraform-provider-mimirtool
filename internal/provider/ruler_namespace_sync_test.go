// Copyright 2026 OVHcloud
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"

	"github.com/grafana/mimir/pkg/mimirtool/rules/rwrulefmt"
)

type fakeRulerClient struct {
	mimirClientInterface
	groups  map[string][]rwrulefmt.RuleGroup
	failOn  string
	listErr error
	deleted []string
}

func (f *fakeRulerClient) ListRules(_ context.Context, namespace string) (map[string][]rwrulefmt.RuleGroup, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	// Mimir answers a namespace it does not hold with 200 and {}, not 404.
	groups, ok := f.groups[namespace]
	if !ok {
		return map[string][]rwrulefmt.RuleGroup{}, nil
	}
	return map[string][]rwrulefmt.RuleGroup{namespace: groups}, nil
}

func (f *fakeRulerClient) CreateRuleGroup(_ context.Context, namespace string, rg rwrulefmt.RuleGroup) error {
	if rg.Name == f.failOn {
		return errors.New("create failed")
	}
	for i, group := range f.groups[namespace] {
		if group.Name == rg.Name {
			f.groups[namespace][i] = rg
			return nil
		}
	}
	f.groups[namespace] = append(f.groups[namespace], rg)
	return nil
}

func (f *fakeRulerClient) DeleteRuleGroup(_ context.Context, namespace, groupName string) error {
	f.deleted = append(f.deleted, groupName)
	kept := f.groups[namespace][:0]
	for _, group := range f.groups[namespace] {
		if group.Name != groupName {
			kept = append(kept, group)
		}
	}
	f.groups[namespace] = kept
	return nil
}

func ruleGroups(names ...string) []rwrulefmt.RuleGroup {
	groups := make([]rwrulefmt.RuleGroup, 0, len(names))
	for _, name := range names {
		var group rwrulefmt.RuleGroup
		group.Name = name
		groups = append(groups, group)
	}
	return groups
}

func groupNames(groups []rwrulefmt.RuleGroup) []string {
	names := make([]string, 0, len(groups))
	for _, group := range groups {
		names = append(names, group.Name)
	}
	sort.Strings(names)
	return names
}

func TestSyncRuleGroups(t *testing.T) {
	tests := []struct {
		name        string
		remote      map[string][]rwrulefmt.RuleGroup
		configured  []string
		failOn      string
		listErr     error
		wantErr     bool
		wantGroups  []string
		wantDeleted []string
	}{
		{
			name:       "empty namespace",
			remote:     map[string][]rwrulefmt.RuleGroup{},
			configured: []string{"a", "b"},
			wantGroups: []string{"a", "b"},
		},
		{
			name:        "unconfigured group removed",
			remote:      map[string][]rwrulefmt.RuleGroup{"ns": ruleGroups("a", "extra")},
			configured:  []string{"a"},
			wantGroups:  []string{"a"},
			wantDeleted: []string{"extra"},
		},
		{
			name:       "nothing deleted when a create fails",
			remote:     map[string][]rwrulefmt.RuleGroup{"ns": ruleGroups("old")},
			configured: []string{"a", "b"},
			failOn:     "b",
			wantErr:    true,
			wantGroups: []string{"a", "old"},
		},
		{
			name:       "nothing created when the list fails",
			remote:     map[string][]rwrulefmt.RuleGroup{"ns": ruleGroups("old")},
			configured: []string{"a"},
			listErr:    errors.New("list failed"),
			wantErr:    true,
			wantGroups: []string{"old"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeRulerClient{groups: tt.remote, failOn: tt.failOn, listErr: tt.listErr}
			err := syncRuleGroups(context.Background(), fake, "ns", ruleGroups(tt.configured...))
			if (err != nil) != tt.wantErr {
				t.Fatalf("syncRuleGroups() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got := groupNames(fake.groups["ns"]); !reflect.DeepEqual(got, tt.wantGroups) {
				t.Errorf("groups = %v, want %v", got, tt.wantGroups)
			}
			if !reflect.DeepEqual(fake.deleted, tt.wantDeleted) {
				t.Errorf("deleted = %v, want %v", fake.deleted, tt.wantDeleted)
			}
		})
	}
}
