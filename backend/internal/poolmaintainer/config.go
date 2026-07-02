package poolmaintainer

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	LocalSub2API      LocalSub2APIConfig       `yaml:"local_sub2api"`
	Policy            PolicyConfig             `yaml:"policy"`
	Upstreams         []UpstreamConfig         `yaml:"upstreams"`
	Accounts          []AccountRuleConfig      `yaml:"accounts"`
	SelfBuiltAccounts []SelfBuiltAccountConfig `yaml:"self_built_accounts"`
}

type LocalSub2APIConfig struct {
	BaseURL       string `yaml:"base_url"`
	AdminTokenEnv string `yaml:"admin_token_env"`
}

type PolicyConfig struct {
	SafetyMargin  float64           `yaml:"safety_margin"`
	SelfBuiltRate float64           `yaml:"self_built_rate"`
	SalesGroups   []SaleGroupConfig `yaml:"sales_groups"`
	Priority      PriorityConfig    `yaml:"priority"`
}

type SaleGroupConfig struct {
	Name    string  `yaml:"name" json:"name"`
	GroupID int64   `yaml:"group_id" json:"group_id"`
	Rate    float64 `yaml:"rate" json:"rate"`
}

type PriorityConfig struct {
	SelfBuilt     int `yaml:"self_built"`
	UpstreamStart int `yaml:"upstream_start"`
	UpstreamStep  int `yaml:"upstream_step"`
}

type UpstreamConfig struct {
	ID               string              `yaml:"id"`
	BaseURL          string              `yaml:"base_url"`
	PricingPageURL   string              `yaml:"pricing_page_url"`
	BrowserProfile   string              `yaml:"browser_profile"`
	GroupNameAliases map[string][]string `yaml:"group_name_aliases"`
}

type AccountRuleConfig struct {
	MatchName          string   `yaml:"match_name"`
	UpstreamID         string   `yaml:"upstream_id"`
	UpstreamGroup      string   `yaml:"upstream_group"`
	AllowedSalesGroups []string `yaml:"allowed_sales_groups"`
}

type SelfBuiltAccountConfig struct {
	MatchName          string   `yaml:"match_name"`
	AllowedSalesGroups []string `yaml:"allowed_sales_groups"`
}

func LoadConfig(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) Validate() error {
	if c == nil {
		return errors.New("config is nil")
	}
	c.LocalSub2API.BaseURL = strings.TrimSpace(c.LocalSub2API.BaseURL)
	c.LocalSub2API.AdminTokenEnv = strings.TrimSpace(c.LocalSub2API.AdminTokenEnv)
	if c.LocalSub2API.BaseURL == "" {
		return errors.New("local_sub2api.base_url is required")
	}
	if c.LocalSub2API.AdminTokenEnv == "" {
		return errors.New("local_sub2api.admin_token_env is required")
	}
	if c.Policy.SafetyMargin == 0 {
		c.Policy.SafetyMargin = 0.02
	}
	if c.Policy.SafetyMargin < 0 {
		return errors.New("policy.safety_margin must be >= 0")
	}
	if c.Policy.Priority.SelfBuilt == 0 {
		c.Policy.Priority.SelfBuilt = 1
	}
	if c.Policy.Priority.UpstreamStart == 0 {
		c.Policy.Priority.UpstreamStart = 5
	}
	if c.Policy.Priority.UpstreamStep == 0 {
		c.Policy.Priority.UpstreamStep = 5
	}
	if c.Policy.Priority.UpstreamStep < 0 {
		return errors.New("policy.priority.upstream_step must be >= 0")
	}
	if len(c.Policy.SalesGroups) == 0 {
		return errors.New("policy.sales_groups is required")
	}

	salesGroups := make(map[string]struct{}, len(c.Policy.SalesGroups))
	for i := range c.Policy.SalesGroups {
		g := &c.Policy.SalesGroups[i]
		g.Name = strings.TrimSpace(g.Name)
		if g.Name == "" {
			return fmt.Errorf("policy.sales_groups[%d].name is required", i)
		}
		if g.GroupID <= 0 {
			return fmt.Errorf("policy.sales_groups[%s].group_id must be > 0", g.Name)
		}
		if g.Rate <= 0 {
			return fmt.Errorf("policy.sales_groups[%s].rate must be > 0", g.Name)
		}
		if _, exists := salesGroups[g.Name]; exists {
			return fmt.Errorf("duplicate sales group name %q", g.Name)
		}
		salesGroups[g.Name] = struct{}{}
	}

	upstreams := make(map[string]struct{}, len(c.Upstreams))
	for i := range c.Upstreams {
		u := &c.Upstreams[i]
		u.ID = strings.TrimSpace(u.ID)
		u.BaseURL = strings.TrimSpace(u.BaseURL)
		u.PricingPageURL = strings.TrimSpace(u.PricingPageURL)
		u.BrowserProfile = strings.TrimSpace(u.BrowserProfile)
		if u.ID == "" {
			return fmt.Errorf("upstreams[%d].id is required", i)
		}
		if _, exists := upstreams[u.ID]; exists {
			return fmt.Errorf("duplicate upstream id %q", u.ID)
		}
		if u.BaseURL == "" {
			return fmt.Errorf("upstreams[%s].base_url is required", u.ID)
		}
		if u.PricingPageURL == "" {
			u.PricingPageURL = u.BaseURL
		}
		upstreams[u.ID] = struct{}{}
	}

	for i := range c.Accounts {
		r := &c.Accounts[i]
		r.MatchName = strings.TrimSpace(r.MatchName)
		r.UpstreamID = strings.TrimSpace(r.UpstreamID)
		r.UpstreamGroup = strings.TrimSpace(r.UpstreamGroup)
		if r.MatchName == "" {
			return fmt.Errorf("accounts[%d].match_name is required", i)
		}
		if r.UpstreamID == "" {
			return fmt.Errorf("accounts[%s].upstream_id is required", r.MatchName)
		}
		if _, exists := upstreams[r.UpstreamID]; !exists {
			return fmt.Errorf("accounts[%s] references unknown upstream id %q", r.MatchName, r.UpstreamID)
		}
		if r.UpstreamGroup == "" {
			return fmt.Errorf("accounts[%s].upstream_group is required", r.MatchName)
		}
		if err := validateAllowedSalesGroups(r.MatchName, r.AllowedSalesGroups, salesGroups); err != nil {
			return err
		}
	}

	for i := range c.SelfBuiltAccounts {
		r := &c.SelfBuiltAccounts[i]
		r.MatchName = strings.TrimSpace(r.MatchName)
		if r.MatchName == "" {
			return fmt.Errorf("self_built_accounts[%d].match_name is required", i)
		}
		if err := validateAllowedSalesGroups(r.MatchName, r.AllowedSalesGroups, salesGroups); err != nil {
			return err
		}
	}

	return nil
}

func validateAllowedSalesGroups(owner string, allowed []string, salesGroups map[string]struct{}) error {
	if len(allowed) == 0 {
		return fmt.Errorf("%s allowed_sales_groups is required", owner)
	}
	for _, name := range allowed {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			return fmt.Errorf("%s allowed_sales_groups contains an empty name", owner)
		}
		if _, exists := salesGroups[trimmed]; !exists {
			return fmt.Errorf("%s references unknown allowed sales group %q", owner, trimmed)
		}
	}
	return nil
}

func (c *Config) SaleGroupByName(name string) (SaleGroupConfig, bool) {
	if c == nil {
		return SaleGroupConfig{}, false
	}
	for _, g := range c.Policy.SalesGroups {
		if g.Name == name {
			return g, true
		}
	}
	return SaleGroupConfig{}, false
}

func (c *Config) UpstreamByID(id string) (UpstreamConfig, bool) {
	if c == nil {
		return UpstreamConfig{}, false
	}
	for _, u := range c.Upstreams {
		if u.ID == id {
			return u, true
		}
	}
	return UpstreamConfig{}, false
}
