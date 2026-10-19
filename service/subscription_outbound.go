package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/deposist/s-ui-x-extended/core"
	"github.com/deposist/s-ui-x-extended/database"
	"github.com/deposist/s-ui-x-extended/database/model"
	"github.com/deposist/s-ui-x-extended/realtime"
	"github.com/deposist/s-ui-x-extended/util"
	"github.com/deposist/s-ui-x-extended/util/common"
)

const SubscriptionType = "subscription"
const subscriptionTagPrefix = "__subscription_"

type subscriptionOptions struct {
	URL       string `json:"url"`
	UserAgent string `json:"user_agent,omitempty"`
	util.SubscriptionDevice
	Primary        string           `json:"primary"`
	Backup         string           `json:"backup,omitempty"`
	UpdateInterval string           `json:"update_interval,omitempty"`
	Failover       failoverProbe    `json:"failover,omitempty"`
	Nodes          []map[string]any `json:"nodes"`
	UpdatedAt      int64            `json:"updated_at"`
	CheckedAt      int64            `json:"checked_at,omitempty"`
	LastError      string           `json:"last_error,omitempty"`
}

func subscriptionMemberTags(row model.Outbound, opts subscriptionOptions) []string {
	hash := sha256.Sum256([]byte(row.Tag))
	prefix := fmt.Sprintf("%s%x_", subscriptionTagPrefix, hash[:12])
	tags := []string{prefix + "primary"}
	if opts.Backup != "" {
		tags = append(tags, prefix+"backup")
	}
	return tags
}

func parseSubscriptionOptions(raw json.RawMessage) (subscriptionOptions, error) {
	var opts subscriptionOptions
	if err := json.Unmarshal(raw, &opts); err != nil {
		return opts, common.NewError("invalid subscription options")
	}
	if strings.TrimSpace(opts.URL) == "" || strings.TrimSpace(opts.Primary) == "" {
		return opts, common.NewError("subscription URL and primary server are required")
	}
	if opts.Backup != "" && opts.Backup == opts.Primary {
		return opts, common.NewError("primary and backup must be different servers")
	}
	if opts.UserAgent == "" {
		opts.UserAgent = "sing-box"
	}
	if opts.UpdateInterval == "" {
		opts.UpdateInterval = "24h"
	}
	interval, err := time.ParseDuration(opts.UpdateInterval)
	if err != nil || interval < time.Minute {
		return opts, common.NewError("subscription update interval must be at least 1m")
	}
	if err := validateProbeTarget(opts.Failover.resolvedTarget()); err != nil {
		return opts, err
	}
	if opts.Failover.Interval != "" {
		d, err := time.ParseDuration(opts.Failover.Interval)
		if err != nil || d < MinInterval {
			return opts, common.NewError("probe interval must be at least 5s")
		}
	}
	if opts.Failover.Hysteresis < 0 {
		return opts, common.NewError("hysteresis must be positive")
	}
	if opts.Failover.AllDownPolicy != "" && opts.Failover.AllDownPolicy != AllDownPolicyHoldCurrent {
		return opts, common.NewError("subscription failover must not bypass the selected servers")
	}
	return opts, nil
}

func prepareSubscription(ctx context.Context, row *model.Outbound) error {
	opts, err := parseSubscriptionOptions(row.Options)
	if err != nil {
		return err
	}
	nodes, err := util.GetExternalSubContext(ctx, opts.URL, opts.UserAgent, opts.SubscriptionDevice)
	if err != nil {
		return common.NewError("subscription download or parsing failed; check URL, access and format")
	}
	selected := make([]map[string]any, 0, 2)
	for _, name := range []string{opts.Primary, opts.Backup} {
		if name == "" {
			continue
		}
		var picked map[string]any
		for _, node := range nodes {
			if node["tag"] != name {
				continue
			}
			if picked != nil {
				return common.NewError("selected server name is ambiguous in subscription")
			}
			picked = node
		}
		if picked == nil {
			return common.NewError("selected server is missing from subscription; cached configuration retained")
		}
		if err := validateSubscriptionNode(picked); err != nil {
			return err
		}
		selected = append(selected, picked)
	}
	opts.Nodes = selected
	opts.UpdatedAt = time.Now().Unix()
	opts.CheckedAt = opts.UpdatedAt
	opts.LastError = ""
	row.Options, err = json.Marshal(opts)
	if err != nil {
		return err
	}
	configs, err := assembleSubscription(*row)
	if err != nil {
		return err
	}
	config, err := json.Marshal(map[string]any{"log": map[string]any{"disabled": true}, "outbounds": configs})
	if err != nil {
		return err
	}
	if err := core.ValidateConfig(config); err != nil {
		return common.NewError("selected server configuration is not supported by this core")
	}
	return nil
}

func validateSubscriptionNode(node map[string]any) error {
	switch node["type"] {
	case "socks", "http", "shadowsocks", "vmess", "vless", "trojan", "hysteria", "hysteria2", "tuic", "anytls", "ssh", "shadowtls", "naive", "mieru", "sudoku", "trusttunnel":
	default:
		return common.NewError("selected subscription protocol is not supported")
	}
	var check func(any) bool
	check = func(value any) bool {
		switch value := value.(type) {
		case map[string]any:
			for key, v := range value {
				if key == "detour" || key == "domain_resolver" || key == "bind_interface" || key == "routing_mark" || key == "netns" || key == "protect_path" || strings.HasSuffix(key, "_path") || strings.HasSuffix(key, "_file") {
					return false
				}
				if !check(v) {
					return false
				}
			}
		case []any:
			for _, v := range value {
				if !check(v) {
					return false
				}
			}
		}
		return true
	}
	if !check(node) {
		return common.NewError("subscription servers must be self-contained and cannot reference local files or outbound detours")
	}
	return nil
}

func assembleSubscription(row model.Outbound) ([]json.RawMessage, error) {
	opts, err := parseSubscriptionOptions(row.Options)
	if err != nil {
		return nil, err
	}
	tags := subscriptionMemberTags(row, opts)
	if len(opts.Nodes) != len(tags) {
		return nil, common.NewError("subscription has no saved server configuration; reload and save it")
	}
	configs := make([]json.RawMessage, 0, len(tags)+1)
	for i, node := range opts.Nodes {
		copy := make(map[string]any, len(node))
		for k, v := range node {
			copy[k] = v
		}
		copy["tag"] = tags[i]
		raw, err := json.Marshal(copy)
		if err != nil {
			return nil, err
		}
		configs = append(configs, raw)
	}
	selector, err := json.Marshal(map[string]any{"type": "selector", "tag": row.Tag, "outbounds": tags, "default": tags[0], "interrupt_exist_connections": false})
	if err != nil {
		return nil, err
	}
	return append(configs, selector), nil
}

// RefreshSubscriptions never replaces a selected server by position or by another name.
func (s *ConfigService) RefreshSubscriptions(ctx context.Context, force bool) error {
	db := database.GetDB()
	var rows []model.Outbound
	if err := db.Where("type = ?", SubscriptionType).Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return err
		}
		opts, err := parseSubscriptionOptions(row.Options)
		if err != nil {
			continue
		}
		interval, _ := time.ParseDuration(opts.UpdateInterval)
		if !force && time.Since(time.Unix(opts.CheckedAt, 0)) < interval {
			continue
		}
		fresh := row
		err = prepareSubscription(ctx, &fresh)
		changed := false
		if err != nil {
			opts.LastError = err.Error()
			opts.CheckedAt = time.Now().Unix()
			fresh.Options, err = json.Marshal(opts)
			if err != nil {
				return err
			}
		} else {
			var next subscriptionOptions
			if err = json.Unmarshal(fresh.Options, &next); err != nil {
				return err
			}
			before, _ := json.Marshal(opts.Nodes)
			after, _ := json.Marshal(next.Nodes)
			changed = !bytes.Equal(before, after)
		}
		result := db.Model(&model.Outbound{}).Where("id = ? AND options = ?", row.Id, row.Options).Update("options", fresh.Options)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			continue
		} // A concurrent edit/deletion wins over this download.
		s.setLastUpdate(time.Now().Unix())
		realtime.Publish(realtime.TopicConfigInvalidated, nil)
		if changed {
			s.applyPostCommitCoreChanges(postCommitCorePlan{needsCoreRestart: true, restartReason: "subscription server configuration updated"})
		}
	}
	return nil
}
