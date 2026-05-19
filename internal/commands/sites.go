// Package commands — `aegean sites` subcommand.
//
// One-shot deploy is the headline use case:
//
//	aegean sites deploy ./dist                  # zip + POST to the active site
//	aegean sites deploy ./dist --slug shop      # explicit Site
//	aegean sites deploy ./dist.zip              # pre-built zip
//
// CRUD + history round it out:
//
//	aegean sites list
//	aegean sites create --slug shop --bucket aegean-production-user-platform
//	aegean sites delete <slug-or-id>
//	aegean sites history <slug-or-id>
//	aegean sites activate <slug-or-id> <deployment-id>
//
// All commands authenticate via the JWT stored at ~/.aegean/credentials.
package commands

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Aegean-Robotics/aegean-cli/internal/client"
	"github.com/Aegean-Robotics/aegean-cli/internal/output"
	"github.com/spf13/cobra"
)

// maxBundleBytes mirrors SiteDeploymentService.MAX_BUNDLE_BYTES on the
// backend. We pre-check client-side so the user gets a clear error
// before burning upload bandwidth on a payload the API will reject.
const maxBundleBytes int64 = 250 * 1024 * 1024

func newSitesCmd(flags *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sites",
		Short: "Manage static sites (deploy bundles, rollback, list)",
		Long: "aegean sites — register, deploy, and roll back static-site bundles served from " +
			"your SeaweedFS user buckets. Each Site auto-publishes at both " +
			"/sites/<alias>/<slug>/ (path URL) and <alias>-<slug>.sites.aegeanengine.com " +
			"(wildcard subdomain, once provisioned). See " +
			"todo/sites-subdomain-support.md for the full architecture.",
	}
	cmd.AddCommand(
		newSitesListCmd(flags),
		newSitesCreateCmd(flags),
		newSitesDeleteCmd(flags),
		newSitesDeployCmd(flags),
		newSitesHistoryCmd(flags),
		newSitesActivateCmd(flags),
	)
	return cmd
}

// ── list ───────────────────────────────────────────────────────────────────

func newSitesListCmd(flags *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List Sites on the current account",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sess, err := loadSession(flags)
			if err != nil {
				return err
			}
			if err := sess.requireToken(); err != nil {
				return err
			}
			sites, err := sess.client.ListSites(context.Background())
			if err != nil {
				return err
			}
			switch sess.cfg.Output {
			case output.FormatJSON:
				return output.JSON(cmd.OutOrStdout(), sites)
			case output.FormatYAML:
				return output.YAML(cmd.OutOrStdout(), sites)
			}
			if len(sites) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(),
					"No sites yet. Create one with `aegean sites create --slug <name> --bucket <bucket>`.")
				return nil
			}
			rows := make([][]string, len(sites))
			for i, s := range sites {
				preferred := s.PreferredSubdomain
				if preferred == "" {
					preferred = "(derived)"
				}
				enabled := "yes"
				if !s.Enabled {
					enabled = "no"
				}
				rows[i] = []string{s.Slug, s.BucketName, preferred, enabled, s.CreatedAt.Format("2006-01-02")}
			}
			return output.Table(cmd.OutOrStdout(),
				[]string{"SLUG", "BUCKET", "PREFERRED", "ENABLED", "CREATED"},
				rows,
			)
		},
	}
}

// ── create ─────────────────────────────────────────────────────────────────

func newSitesCreateCmd(flags *GlobalFlags) *cobra.Command {
	var (
		slug, bucket, prefix, indexDoc, errorDoc string
		preferred, customDomain                  string
		spaFallback                              bool
		cacheMaxAge                              int
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Register a new static site",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sess, err := loadSession(flags)
			if err != nil {
				return err
			}
			if err := sess.requireToken(); err != nil {
				return err
			}
			if slug == "" || bucket == "" {
				return fmt.Errorf("both --slug and --bucket are required")
			}

			in := client.SiteInput{
				Slug:         slug,
				BucketName:   bucket,
				KeyPrefix:    prefix,
				IndexDocument: indexDoc,
				ErrorDocument: errorDoc,
				CustomDomain:       customDomain,
				PreferredSubdomain: preferred,
			}
			// Pointer fields so unset != false on the wire.
			if cmd.Flags().Changed("spa-fallback") {
				in.SpaFallback = &spaFallback
			}
			if cmd.Flags().Changed("cache-max-age") {
				in.DefaultCacheMaxAge = &cacheMaxAge
			}

			site, err := sess.client.CreateSite(context.Background(), in)
			if err != nil {
				return err
			}
			switch sess.cfg.Output {
			case output.FormatJSON:
				return output.JSON(cmd.OutOrStdout(), site)
			case output.FormatYAML:
				return output.YAML(cmd.OutOrStdout(), site)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"✓ Created site %q (id %s)\n  bucket: %s\n  deploy: aegean sites deploy ./dist --slug %s\n",
				site.Slug, site.ID, site.BucketName, site.Slug,
			)
			return nil
		},
	}
	cmd.Flags().StringVar(&slug, "slug", "", "URL slug (lowercase, dashes, 1-64 chars) — REQUIRED")
	cmd.Flags().StringVar(&bucket, "bucket", "", "Bucket name (must already include the env's S3_BUCKET_PREFIX) — REQUIRED")
	cmd.Flags().StringVar(&prefix, "prefix", "", "Optional key prefix inside the bucket (default: bucket root)")
	cmd.Flags().StringVar(&indexDoc, "index", "index.html", "Index document")
	cmd.Flags().StringVar(&errorDoc, "error", "404.html", "Custom error document")
	cmd.Flags().BoolVar(&spaFallback, "spa-fallback", false, "Treat HTML-accept misses as deep links → serve index.html (200)")
	cmd.Flags().IntVar(&cacheMaxAge, "cache-max-age", 300, "Default Cache-Control max-age in seconds")
	cmd.Flags().StringVar(&preferred, "preferred-subdomain", "", "Phase 4b: pick the leftmost label on *.sites.aegeanengine.com (overrides the derived <alias>-<slug>)")
	cmd.Flags().StringVar(&customDomain, "custom-domain", "", "Optional BYO domain (e.g. shop.example.com) — needs operator-side DNS+TLS")
	_ = cmd.MarkFlagRequired("slug")
	_ = cmd.MarkFlagRequired("bucket")
	return cmd
}

// ── delete ─────────────────────────────────────────────────────────────────

func newSitesDeleteCmd(flags *GlobalFlags) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <slug-or-id>",
		Short: "Delete a site (does not free the underlying bucket objects)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sess, err := loadSession(flags)
			if err != nil {
				return err
			}
			if err := sess.requireToken(); err != nil {
				return err
			}
			siteID, slug, err := resolveSiteRef(sess.client, args[0])
			if err != nil {
				return err
			}
			if !yes {
				fmt.Fprintf(cmd.OutOrStdout(), "Delete site %s (id %s)? Type 'yes' to confirm: ", slug, siteID)
				scanner := bufio.NewScanner(cmd.InOrStdin())
				if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "yes" {
					fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
					return nil
				}
			}
			if err := sess.client.DeleteSite(context.Background(), siteID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Deleted site %s.\n", slug)
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the confirmation prompt")
	return cmd
}

// ── deploy ─────────────────────────────────────────────────────────────────

func newSitesDeployCmd(flags *GlobalFlags) *cobra.Command {
	var (
		slug, notes string
	)
	cmd := &cobra.Command{
		Use:   "deploy <dir-or-zip>",
		Short: "Zip a directory (or use a pre-built zip) and atomically deploy it as the active bundle",
		Long: "If <dir-or-zip> is a directory, the CLI zips it on the fly (excluding .git, " +
			"node_modules, .DS_Store, *.swp) and POSTs the result. If it's a .zip file, it's " +
			"uploaded as-is. The backend extracts under a versioned prefix and flips the active " +
			"pointer in one SQL UPDATE — old deployments stay reachable via `aegean sites history`.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sess, err := loadSession(flags)
			if err != nil {
				return err
			}
			if err := sess.requireToken(); err != nil {
				return err
			}

			// Resolve the Site. If --slug wasn't passed and exactly
			// one site exists, default to that. Multi-site accounts
			// must be explicit.
			if slug == "" {
				all, err := sess.client.ListSites(context.Background())
				if err != nil {
					return err
				}
				switch len(all) {
				case 0:
					return fmt.Errorf("no sites on this account — create one first: aegean sites create --slug <name> --bucket <bucket>")
				case 1:
					slug = all[0].Slug
				default:
					return fmt.Errorf("multiple sites on this account — pass --slug <name> to disambiguate")
				}
			}
			siteID, slugUsed, err := resolveSiteRef(sess.client, slug)
			if err != nil {
				return err
			}

			// Build (or read) the zip.
			path := args[0]
			info, err := os.Stat(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			var zipBytes []byte
			var zipName string
			if info.IsDir() {
				zipBytes, err = zipDirectory(path)
				if err != nil {
					return err
				}
				zipName = filepath.Base(filepath.Clean(path)) + ".zip"
				if zipName == "..zip" || zipName == "./zip" || zipName == ".zip" {
					zipName = "bundle.zip"
				}
			} else {
				zipBytes, err = os.ReadFile(path)
				if err != nil {
					return fmt.Errorf("read %s: %w", path, err)
				}
				zipName = filepath.Base(path)
				if !strings.HasSuffix(strings.ToLower(zipName), ".zip") {
					return fmt.Errorf("%s is not a directory and doesn't end in .zip — pass a built bundle or a source directory", path)
				}
			}

			// Client-side size precheck — mirrors the backend's 250 MB
			// MAX_BUNDLE_BYTES so the user gets a clear error instead of
			// burning bandwidth on an upload the API will 4xx.
			if int64(len(zipBytes)) > maxBundleBytes {
				return fmt.Errorf("bundle is %s, exceeds the 250 MB backend cap — split or trim before retrying",
					humanBytes(int64(len(zipBytes))))
			}

			fmt.Fprintf(cmd.OutOrStdout(),
				"→ deploying %s (%s) to site %s\n",
				zipName, humanBytes(int64(len(zipBytes))), slugUsed,
			)

			dep, err := sess.client.DeploySite(context.Background(), siteID, zipBytes, zipName, notes)
			if err != nil {
				return err
			}

			switch sess.cfg.Output {
			case output.FormatJSON:
				return output.JSON(cmd.OutOrStdout(), dep)
			case output.FormatYAML:
				return output.YAML(cmd.OutOrStdout(), dep)
			}

			// Try to print the real path URL. Best-effort — if the
			// /v1/account fetch fails (older backend, token quirk), we
			// fall back to <your-alias> + a hint.
			alias, aliasErr := fetchAccountAlias(sess.client)
			endpointHost := strings.TrimPrefix(strings.TrimPrefix(sess.cfg.Endpoint, "https://"), "http://")
			browserHost := strings.Replace(endpointHost, "api.", "", 1)
			if browserHost == endpointHost {
				// Endpoint doesn't follow the api.<env> convention (local
				// docker compose, custom dev box). Stick with the raw form.
				browserHost = endpointHost
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"✓ deployment %s — %d files, %s\n",
				dep.ID, dep.FileCount, humanBytes(dep.BytesTotal),
			)
			if aliasErr == nil && alias != "" {
				wildcardLabel := alias + "-" + slugUsed
				fmt.Fprintf(cmd.OutOrStdout(),
					"  path URL    : https://%s/sites/%s/%s/\n"+
						"  wildcard URL: https://%s.sites.%s/  (live once *.sites.* DNS+TLS is provisioned)\n",
					browserHost, url.PathEscape(alias), url.PathEscape(slugUsed),
					wildcardLabel, strings.TrimPrefix(browserHost, ""),
				)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(),
					"  open: https://%s/sites/<your-alias>/%s/\n"+
						"        (or the wildcard URL once *.sites.* DNS+TLS is provisioned)\n",
					browserHost, slugUsed,
				)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&slug, "slug", "", "Site slug or id (defaults to the only site on the account, if there's one)")
	cmd.Flags().StringVar(&notes, "notes", "", "Free-text label saved with the deployment (shows in history)")
	return cmd
}

// ── history ────────────────────────────────────────────────────────────────

func newSitesHistoryCmd(flags *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "history <slug-or-id>",
		Short: "List deployment history for a site (newest first)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sess, err := loadSession(flags)
			if err != nil {
				return err
			}
			if err := sess.requireToken(); err != nil {
				return err
			}
			siteID, _, err := resolveSiteRef(sess.client, args[0])
			if err != nil {
				return err
			}
			deployments, err := sess.client.ListSiteDeployments(context.Background(), siteID)
			if err != nil {
				return err
			}
			switch sess.cfg.Output {
			case output.FormatJSON:
				return output.JSON(cmd.OutOrStdout(), deployments)
			case output.FormatYAML:
				return output.YAML(cmd.OutOrStdout(), deployments)
			}
			if len(deployments) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(),
					"No deployments yet. Run `aegean sites deploy ./dist`.")
				return nil
			}
			rows := make([][]string, len(deployments))
			for i, d := range deployments {
				notes := d.Notes
				if len(notes) > 40 {
					notes = notes[:40] + "…"
				}
				rows[i] = []string{
					d.ID,
					d.CreatedAt.Format("2006-01-02 15:04"),
					fmt.Sprintf("%d", d.FileCount),
					fmt.Sprintf("%d", d.BytesTotal),
					notes,
				}
			}
			return output.Table(cmd.OutOrStdout(),
				[]string{"ID", "WHEN", "FILES", "BYTES", "NOTES"},
				rows,
			)
		},
	}
}

// ── activate (rollback / roll-forward) ─────────────────────────────────────

func newSitesActivateCmd(flags *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "activate <slug-or-id> <deployment-id>",
		Short: "Flip a site to a previous deployment (rollback) or a newer one (roll-forward)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sess, err := loadSession(flags)
			if err != nil {
				return err
			}
			if err := sess.requireToken(); err != nil {
				return err
			}
			siteID, slug, err := resolveSiteRef(sess.client, args[0])
			if err != nil {
				return err
			}
			deploymentID := args[1]
			if err := sess.client.ActivateDeployment(context.Background(), siteID, deploymentID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"✓ Activated deployment %s on site %s. The change is live now.\n",
				deploymentID, slug,
			)
			return nil
		},
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

// resolveSiteRef takes a CLI argument (UUID or slug) and returns (id, slug).
// UUIDs short-circuit (no list hop). Slugs trigger a /api/sites list +
// client-side filter — fine because tenants rarely have many sites.
func resolveSiteRef(c *client.Client, ref string) (id, slug string, err error) {
	// 32+ hyphen UUIDs treat as id; anything else as slug. The server
	// is the actual source of truth; this is just a friendly shortcut.
	if len(ref) == 36 && strings.Count(ref, "-") == 4 {
		return ref, ref, nil
	}
	s, err := c.FindSite(context.Background(), ref)
	if err != nil {
		return "", "", err
	}
	return s.ID, s.Slug, nil
}

// zipDirectory walks `root` and returns an in-memory zip. Skips common
// noise (.git, node_modules, .DS_Store, *.swp). The 250 MB cap on the
// backend means we don't need a per-file size cap here — backend rejects
// over-large bundles with a clear error.
func zipDirectory(root string) ([]byte, error) {
	root = filepath.Clean(root)
	rootInfo, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !rootInfo.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	skipDirs := map[string]struct{}{
		".git":         {},
		"node_modules": {},
		".DS_Store":    {},
	}

	walkErr := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		base := filepath.Base(p)
		if _, skip := skipDirs[base]; skip {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(base, ".swp") || base == ".DS_Store" {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		// Zip entries always use forward slashes.
		entryName := filepath.ToSlash(rel)
		zh := &zip.FileHeader{
			Name:     entryName,
			Method:   zip.Deflate,
			Modified: info.ModTime(),
		}
		w, err := zw.CreateHeader(zh)
		if err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(w, f); err != nil {
			return err
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// fetchAccountAlias hits GET /v1/account and returns the alias. Best-effort —
// the deploy command falls back to a placeholder when this errors so a
// missing endpoint (older backend) or quirky token doesn't break the user's
// happy path.
func fetchAccountAlias(c *client.Client) (string, error) {
	av, err := c.CurrentAccount(context.Background())
	if err != nil {
		return "", err
	}
	if av == nil {
		return "", fmt.Errorf("empty account view")
	}
	return av.Alias, nil
}

// humanBytes formats a byte count as "X.Y MB" / "X KB" / "N B". Used in
// deploy output so a 47 MB bundle reads as "47.0 MB" not "49283072 bytes".
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n2 := n / unit; n2 >= unit; n2 /= unit {
		div *= unit
		exp++
	}
	suffix := []string{"KB", "MB", "GB", "TB"}[exp]
	return fmt.Sprintf("%.1f %s", float64(n)/float64(div), suffix)
}
