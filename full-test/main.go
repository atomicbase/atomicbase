package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type config struct {
	baseURL       string
	apiKey        string
	repoRoot      string
	database      string
	table         string
	token         string
	idColumn      string
	titleColumn   string
	completedCol  string
	seed          int64
	steps         int
	loop          bool
	provision     bool
	keepResources bool
	failOn4xx     bool
	requestTimout time.Duration
}

type provisionedResources struct {
	workspaceDir string
	templateName string
	databaseName string
}

type todo struct {
	ID        int
	Title     string
	Completed bool
}

type client struct {
	httpClient *http.Client
	baseURL    string
	database   string
	table      string
	token      string
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

type op struct {
	name string
	run  func(*rand.Rand, *runState) error
}

type runState struct {
	cfg    config
	client *client
	model  map[int]todo
	step   int
	seed   int64
}

func main() {
	cfg := loadConfig()

	if cfg.token == "" {
		cfg.token = cfg.apiKey
	}

	var resources *provisionedResources
	if cfg.provision {
		provisioned, err := provisionTestResources(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to provision test resources: %v\n", err)
			os.Exit(1)
		}
		resources = provisioned
		cfg.database = resources.databaseName
		fmt.Printf("Provisioned template=%s database=%s\n", resources.templateName, resources.databaseName)

		if !cfg.keepResources {
			defer cleanupProvisionedResources(cfg, resources)
		}
	} else if cfg.database == "" {
		fmt.Fprintln(os.Stderr, "Database is required when -provision=false (set -database or ATOMICBASE_DATABASE)")
		os.Exit(1)
	}

	if cfg.loop {
		run := int64(0)
		for {
			runSeed := cfg.seed + run
			if err := runSimulation(cfg, runSeed); err != nil {
				fmt.Fprintf(os.Stderr, "\nSimulation failed (seed=%d, run=%d): %v\n", runSeed, run, err)
				printReplayHint(cfg, runSeed)
				os.Exit(1)
			}
			run++
		}
	}

	if err := runSimulation(cfg, cfg.seed); err != nil {
		fmt.Fprintf(os.Stderr, "Simulation failed: %v\n", err)
		printReplayHint(cfg, cfg.seed)
		os.Exit(1)
	}
}

func runSimulation(cfg config, seed int64) error {
	r := rand.New(rand.NewSource(seed))
	httpClient := &http.Client{Timeout: cfg.requestTimout}
	c := &client{
		httpClient: httpClient,
		baseURL:    strings.TrimRight(cfg.baseURL, "/"),
		database:   cfg.database,
		table:      cfg.table,
		token:      cfg.token,
	}

	s := &runState{
		cfg:    cfg,
		client: c,
		model:  map[int]todo{},
		seed:   seed,
	}

	if err := s.healthcheck(); err != nil {
		return err
	}

	if err := s.warmupAndValidateTable(); err != nil {
		return err
	}

	ops := []op{
		{name: "insert", run: runInsert},
		{name: "update", run: runUpdate},
		{name: "upsert", run: runUpsert},
		{name: "delete", run: runDelete},
		{name: "select", run: runSelect},
	}

	fmt.Printf("Starting deterministic simulation: steps=%d seed=%d table=%s database=%s\n", cfg.steps, seed, cfg.table, cfg.database)
	for i := 1; i <= cfg.steps; i++ {
		s.step = i

		chosen := weightedOp(r, ops)
		if err := chosen.run(r, s); err != nil {
			return fmt.Errorf("step %d (%s): %w", i, chosen.name, err)
		}

		if err := s.verifySnapshot(); err != nil {
			return fmt.Errorf("step %d (%s): %w", i, chosen.name, err)
		}
	}

	fmt.Printf("Simulation passed: steps=%d seed=%d\n", cfg.steps, seed)
	return nil
}

func loadConfig() config {
	defaultSeed := time.Now().Unix()
	wd, _ := os.Getwd()
	repoRootDefault := envOr("SIM_REPO_ROOT", filepath.Clean(filepath.Join(wd, "..")))

	baseURL := envOr("ATOMICBASE_BASE_URL", "http://localhost:8080")
	apiKey := envOr("ATOMICBASE_API_KEY", "")
	database := envOr("ATOMICBASE_DATABASE", "")
	table := envOr("ATOMICBASE_TABLE", "todos")
	token := envOr("ATOMICBASE_TOKEN", "")
	idCol := envOr("ATOMICBASE_ID_COLUMN", "id")
	titleCol := envOr("ATOMICBASE_TITLE_COLUMN", "title")
	completedCol := envOr("ATOMICBASE_COMPLETED_COLUMN", "completed")
	repoRoot := repoRootDefault

	steps := envIntOr("SIM_STEPS", 500)
	seed := envInt64Or("SIM_SEED", defaultSeed)
	loop := envBoolOr("SIM_LOOP", false)
	provision := envBoolOr("SIM_PROVISION", true)
	keepResources := envBoolOr("SIM_KEEP_RESOURCES", false)
	failOn4xx := envBoolOr("SIM_FAIL_ON_4XX", false)
	timeoutMS := envIntOr("SIM_TIMEOUT_MS", 5000)

	flag.StringVar(&baseURL, "base-url", baseURL, "Atomicbase API base URL")
	flag.StringVar(&apiKey, "api-key", apiKey, "API key for CLI provisioning and API auth")
	flag.StringVar(&repoRoot, "repo-root", repoRoot, "Atomicbase repo root (for invoking CLI)")
	flag.StringVar(&database, "database", database, "Database header value")
	flag.StringVar(&table, "table", table, "Table to test")
	flag.StringVar(&token, "token", token, "Bearer token (optional if API auth disabled)")
	flag.StringVar(&idCol, "id-column", idCol, "ID column")
	flag.StringVar(&titleCol, "title-column", titleCol, "Title/value column")
	flag.StringVar(&completedCol, "completed-column", completedCol, "Completed/boolean column")
	flag.IntVar(&steps, "steps", steps, "Operations per run")
	flag.Int64Var(&seed, "seed", seed, "Deterministic seed")
	flag.BoolVar(&loop, "loop", loop, "Run forever, incrementing seed per run")
	flag.BoolVar(&provision, "provision", provision, "Provision template/database via CLI before simulation")
	flag.BoolVar(&keepResources, "keep-resources", keepResources, "Keep provisioned template/database after run")
	flag.BoolVar(&failOn4xx, "fail-on-4xx", failOn4xx, "Fail when API returns any 4xx")
	flag.IntVar(&timeoutMS, "timeout-ms", timeoutMS, "HTTP timeout in milliseconds")
	flag.Parse()

	return config{
		baseURL:       baseURL,
		apiKey:        apiKey,
		repoRoot:      repoRoot,
		database:      database,
		table:         table,
		token:         token,
		idColumn:      idCol,
		titleColumn:   titleCol,
		completedCol:  completedCol,
		steps:         steps,
		seed:          seed,
		loop:          loop,
		provision:     provision,
		keepResources: keepResources,
		failOn4xx:     failOn4xx,
		requestTimout: time.Duration(timeoutMS) * time.Millisecond,
	}
}

func (s *runState) healthcheck() error {
	req, err := http.NewRequest(http.MethodGet, s.client.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	res, err := s.client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("healthcheck failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("healthcheck status=%d body=%s", res.StatusCode, string(body))
	}
	return nil
}

func (s *runState) warmupAndValidateTable() error {
	body := map[string]any{
		"select": []any{"*"},
		"limit":  1,
	}
	status, raw, _, err := s.client.query("select", body)
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("table warmup failed status=%d response=%s", status, string(raw))
	}
	return nil
}

func runInsert(r *rand.Rand, s *runState) error {
	t := randomTodo(r)
	body := map[string]any{
		"data": []map[string]any{todoToMap(s.cfg, t)},
	}

	status, raw, apiErr, err := s.client.query("insert", body)
	if err != nil {
		return err
	}

	if status >= 400 {
		if shouldFailStatus(s.cfg, status) {
			return fmt.Errorf("insert rejected status=%d error=%+v raw=%s", status, apiErr, string(raw))
		}
		return nil
	}

	s.model[t.ID] = t
	return nil
}

func runUpsert(r *rand.Rand, s *runState) error {
	t := randomTodo(r)
	body := map[string]any{
		"data": []map[string]any{todoToMap(s.cfg, t)},
	}

	status, raw, apiErr, err := s.client.query("insert,on-conflict=replace", body)
	if err != nil {
		return err
	}

	if status >= 400 {
		if shouldFailStatus(s.cfg, status) {
			return fmt.Errorf("upsert rejected status=%d error=%+v raw=%s", status, apiErr, string(raw))
		}
		return nil
	}

	s.model[t.ID] = t
	return nil
}

func runUpdate(r *rand.Rand, s *runState) error {
	id := pickKnownOrRandomID(r, s.model)
	newCompleted := r.Intn(2) == 0
	newTitle := fmt.Sprintf("sim-upd-%d", r.Intn(1_000_000))
	body := map[string]any{
		"data": map[string]any{
			s.cfg.titleColumn:  newTitle,
			s.cfg.completedCol: newCompleted,
		},
		"where": []map[string]any{{
			s.cfg.idColumn: map[string]any{"eq": id},
		}},
	}

	status, raw, apiErr, err := s.client.query("update", body)
	if err != nil {
		return err
	}

	if status >= 400 {
		if shouldFailStatus(s.cfg, status) {
			return fmt.Errorf("update rejected status=%d error=%+v raw=%s", status, apiErr, string(raw))
		}
		return nil
	}

	if curr, ok := s.model[id]; ok {
		curr.Title = newTitle
		curr.Completed = newCompleted
		s.model[id] = curr
	}
	return nil
}

func runDelete(r *rand.Rand, s *runState) error {
	id := pickKnownOrRandomID(r, s.model)
	body := map[string]any{
		"where": []map[string]any{{
			s.cfg.idColumn: map[string]any{"eq": id},
		}},
	}

	status, raw, apiErr, err := s.client.query("delete", body)
	if err != nil {
		return err
	}

	if status >= 400 {
		if shouldFailStatus(s.cfg, status) {
			return fmt.Errorf("delete rejected status=%d error=%+v raw=%s", status, apiErr, string(raw))
		}
		return nil
	}

	delete(s.model, id)
	return nil
}

func runSelect(r *rand.Rand, s *runState) error {
	body := map[string]any{"select": []any{"*"}}
	if r.Intn(3) == 0 {
		id := pickKnownOrRandomID(r, s.model)
		body["where"] = []map[string]any{{
			s.cfg.idColumn: map[string]any{"eq": id},
		}}
	}

	status, raw, apiErr, err := s.client.query("select", body)
	if err != nil {
		return err
	}

	if status >= 400 {
		if shouldFailStatus(s.cfg, status) {
			return fmt.Errorf("select rejected status=%d error=%+v raw=%s", status, apiErr, string(raw))
		}
		return nil
	}

	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		return fmt.Errorf("select decode failed: %w raw=%s", err, string(raw))
	}
	return nil
}

func (s *runState) verifySnapshot() error {
	body := map[string]any{"select": []any{"*"}}
	status, raw, apiErr, err := s.client.query("select", body)
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("snapshot select failed status=%d error=%+v raw=%s", status, apiErr, string(raw))
	}

	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		return fmt.Errorf("snapshot decode failed: %w raw=%s", err, string(raw))
	}

	actual := map[int]todo{}
	for _, row := range rows {
		id, ok := toInt(row[s.cfg.idColumn])
		if !ok {
			continue
		}
		title, _ := row[s.cfg.titleColumn].(string)
		completed, _ := toBool(row[s.cfg.completedCol])
		actual[id] = todo{ID: id, Title: title, Completed: completed}
	}

	if !equalTodoMaps(s.model, actual) {
		return fmt.Errorf("model mismatch\nexpected=%s\nactual=%s", formatTodos(s.model), formatTodos(actual))
	}

	return nil
}

func provisionTestResources(cfg config) (*provisionedResources, error) {
	if cfg.repoRoot == "" {
		return nil, fmt.Errorf("repo root is required for provisioning")
	}

	fullTestDir := filepath.Join(cfg.repoRoot, "full-test")
	if err := os.MkdirAll(fullTestDir, 0o755); err != nil {
		return nil, err
	}

	workspaceDir, err := os.MkdirTemp(fullTestDir, "sim-work-")
	if err != nil {
		return nil, err
	}

	nameSuffix := fmt.Sprintf("%d-%d", cfg.seed, time.Now().Unix())
	templateName := "full-test-template-" + sanitizeName(nameSuffix)
	databaseName := "full-test-db-" + sanitizeName(nameSuffix)

	if err := os.MkdirAll(filepath.Join(workspaceDir, "schemas"), 0o755); err != nil {
		return nil, err
	}

	configFile := fmt.Sprintf(`export default {
  url: %q,
  apiKey: %q,
  schemas: "./schemas",
};
`, cfg.baseURL, cfg.apiKey)

	if err := os.WriteFile(filepath.Join(workspaceDir, "atomicbase.config.ts"), []byte(configFile), 0o644); err != nil {
		return nil, err
	}

	templateImportPath := filepath.ToSlash(filepath.Join(cfg.repoRoot, "packages", "template", "dist", "index.js"))
	schemaPath := filepath.Join(workspaceDir, "schemas", templateName+".schema.ts")
	if err := os.WriteFile(schemaPath, []byte(complexSchemaTS(templateName, templateImportPath, cfg.seed)), 0o644); err != nil {
		return nil, err
	}

	if err := runAtomicbaseCLI(cfg.repoRoot, workspaceDir, "templates", "push", templateName); err != nil {
		return nil, fmt.Errorf("templates push failed: %w", err)
	}

	if err := runAtomicbaseCLI(cfg.repoRoot, workspaceDir, "databases", "create", databaseName, "--template", templateName); err != nil {
		_ = runAtomicbaseCLI(cfg.repoRoot, workspaceDir, "templates", "delete", templateName, "--force")
		return nil, fmt.Errorf("databases create failed: %w", err)
	}

	return &provisionedResources{
		workspaceDir: workspaceDir,
		templateName: templateName,
		databaseName: databaseName,
	}, nil
}

func cleanupProvisionedResources(cfg config, resources *provisionedResources) {
	if resources == nil {
		return
	}

	if err := runAtomicbaseCLI(cfg.repoRoot, resources.workspaceDir, "databases", "delete", resources.databaseName, "--force"); err != nil {
		fmt.Fprintf(os.Stderr, "Cleanup warning (database delete): %v\n", err)
	}

	if err := runAtomicbaseCLI(cfg.repoRoot, resources.workspaceDir, "templates", "delete", resources.templateName, "--force"); err != nil {
		fmt.Fprintf(os.Stderr, "Cleanup warning (template delete): %v\n", err)
	}

	if err := os.RemoveAll(resources.workspaceDir); err != nil {
		fmt.Fprintf(os.Stderr, "Cleanup warning (workspace remove): %v\n", err)
	}
}

func runAtomicbaseCLI(repoRoot, workspaceDir string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Primary path: run the CLI package directly through pnpm in the workspace.
	cliArgs := append([]string{"--filter", "@atomicbase/cli", "exec", "node", "bin/atomicbase.js"}, args...)
	cmd := exec.CommandContext(ctx, "pnpm", cliArgs...)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "INIT_CWD="+workspaceDir)

	out, err := cmd.CombinedOutput()
	if err == nil {
		if len(out) > 0 {
			fmt.Print(string(out))
		}
		return nil
	}

	// Fallback path: invoke the CLI entry file by absolute path.
	fallbackArgs := append([]string{filepath.Join(repoRoot, "packages", "cli", "bin", "atomicbase.js")}, args...)
	fallback := exec.CommandContext(ctx, "node", fallbackArgs...)
	fallback.Dir = workspaceDir
	fallback.Env = append(os.Environ(), "INIT_CWD="+workspaceDir)

	fallbackOut, fallbackErr := fallback.CombinedOutput()
	if fallbackErr != nil {
		return fmt.Errorf("pnpm invocation failed:\n%s\nnode fallback failed: %w\n%s", string(out), fallbackErr, string(fallbackOut))
	}

	if len(fallbackOut) > 0 {
		fmt.Print(string(fallbackOut))
	}

	return nil
}

func complexSchemaTS(templateName, templateImportPath string, seed int64) string {
	r := rand.New(rand.NewSource(seed))

	minDisplayLen := 2 + r.Intn(4)
	minProjectNameLen := 3 + r.Intn(4)
	minTodoTitleLen := 3 + r.Intn(5)
	maxPriority := 5 + r.Intn(5)

	emailCollation := "NOCASE"
	if r.Intn(3) == 0 {
		emailCollation = "RTRIM"
	}

	todoStatuses := []string{"todo", "in_progress", "done"}
	if r.Intn(2) == 0 {
		todoStatuses = append(todoStatuses, "blocked")
	}
	if r.Intn(2) == 0 {
		todoStatuses = append(todoStatuses, "review")
	}

	statusList := make([]string, 0, len(todoStatuses))
	for _, s := range todoStatuses {
		statusList = append(statusList, fmt.Sprintf("'%s'", s))
	}

	todoFTSColumns := "[\"title\", \"description\"]"
	if r.Intn(2) == 0 {
		todoFTSColumns = "[\"title\", \"description\", \"metadata_json\"]"
	}

	optionalTodoCols := ""
	if r.Intn(2) == 0 {
		optionalTodoCols += "\n    archived_at: c.text(),"
	}
	if r.Intn(2) == 0 {
		optionalTodoCols += "\n    deleted_at: c.text(),"
	}
	if r.Intn(2) == 0 {
		optionalTodoCols += "\n    sprint_order: c.integer().notNull().default(0).check(\"sprint_order >= 0\"),"
	}

	extraTables := ""
	if r.Intn(2) == 0 {
		extraTables += `
  project_members: defineTable({
    project_id: c.integer().primaryKey().references("projects.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    user_id: c.integer().primaryKey().references("users.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    permission: c.text().notNull().default("editor").check("permission in ('viewer','editor','admin')"),
    joined_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
  }).index("idx_project_members_user", ["user_id"]),

`
	}
	if r.Intn(2) == 0 {
		extraTables += `
  todo_reactions: defineTable({
    todo_id: c.integer().primaryKey().references("todos.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    user_id: c.integer().primaryKey().references("users.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    emoji: c.text().primaryKey(),
    created_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
  }),

`
	}

	return fmt.Sprintf(`import { defineSchema, defineTable, c, sql } from %q;

export default defineSchema(%q, {
  users: defineTable({
    id: c.integer().primaryKey(),
    email: c.text().notNull().unique().collate(%q),
    display_name: c.text().notNull().check("length(display_name) >= %d"),
    role: c.text().notNull().default("member").check("role in ('owner','admin','member')"),
    profile_json: c.text().notNull().default("{}"),
    created_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
    updated_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
  }).index("idx_users_role", ["role"]),

  workspaces: defineTable({
    id: c.integer().primaryKey(),
    slug: c.text().notNull().unique().collate("NOCASE"),
    name: c.text().notNull(),
    owner_id: c.integer().notNull().references("users.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    created_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
  }).index("idx_workspaces_owner", ["owner_id"]),

  projects: defineTable({
    id: c.integer().primaryKey(),
    workspace_id: c.integer().notNull().references("workspaces.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    owner_id: c.integer().notNull().references("users.id", { onDelete: "RESTRICT", onUpdate: "CASCADE" }),
    name: c.text().notNull().check("length(name) >= %d"),
    description: c.text(),
    status: c.text().notNull().default("active").check("status in ('active','archived')"),
    priority: c.integer().notNull().default(3).check("priority between 1 and %d"),
    budget: c.real().check("budget >= 0"),
    slug: c.text().generatedAs("lower(replace(name, ' ', '-'))", { stored: true }),
    created_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
  })
    .index("idx_projects_workspace", ["workspace_id"])
    .index("idx_projects_status", ["status"])
    .uniqueIndex("idx_projects_workspace_slug", ["workspace_id", "slug"]),

  tags: defineTable({
    id: c.integer().primaryKey(),
    workspace_id: c.integer().notNull().references("workspaces.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    name: c.text().notNull().collate("NOCASE"),
    color: c.text().default("#cccccc").check("length(color) = 7"),
  }).uniqueIndex("idx_tags_workspace_name", ["workspace_id", "name"]),

  project_tags: defineTable({
    project_id: c.integer().primaryKey().references("projects.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    tag_id: c.integer().primaryKey().references("tags.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    added_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
  }),

  todos: defineTable({
    id: c.integer().primaryKey(),
    project_id: c.integer().notNull().references("projects.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    assignee_id: c.integer().references("users.id", { onDelete: "SET NULL", onUpdate: "CASCADE" }),
    title: c.text().notNull().check("length(title) >= %d"),
    description: c.text(),
    status: c.text().notNull().default("todo").check("status in (%s)"),
    priority: c.integer().notNull().default(3).check("priority between 1 and %d"),
    completed: c.integer().notNull().default(0).check("completed in (0,1)"),
    due_at: c.text(),
    estimate_hours: c.real().check("estimate_hours >= 0"),
    metadata_json: c.text().notNull().default("{}"),
    search_text: c.text().generatedAs("coalesce(title,'') || ' ' || coalesce(description,'')", { stored: true }),%s
    created_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
    updated_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
  })
    .index("idx_todos_project", ["project_id"])
    .index("idx_todos_assignee", ["assignee_id"])
    .index("idx_todos_status_priority", ["status", "priority"])
    .fts(%s),

  comments: defineTable({
    id: c.integer().primaryKey(),
    todo_id: c.integer().notNull().references("todos.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    author_id: c.integer().notNull().references("users.id", { onDelete: "RESTRICT", onUpdate: "CASCADE" }),
    body: c.text().notNull().check("length(body) > 0"),
    metadata_json: c.text().notNull().default("{}"),
    created_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
  })
    .index("idx_comments_todo", ["todo_id"])
    .fts(["body"]),

  attachments: defineTable({
    id: c.integer().primaryKey(),
    todo_id: c.integer().notNull().references("todos.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    filename: c.text().notNull(),
    content_type: c.text().notNull(),
    size_bytes: c.integer().notNull().check("size_bytes >= 0"),
    checksum: c.text().notNull(),
    content: c.blob(),
    created_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
  }).index("idx_attachments_todo", ["todo_id"]),

%s  audit_events: defineTable({
    todo_id: c.integer().primaryKey().references("todos.id", { onDelete: "CASCADE", onUpdate: "CASCADE" }),
    seq: c.integer().primaryKey(),
    actor_id: c.integer().references("users.id", { onDelete: "SET NULL", onUpdate: "CASCADE" }),
    action: c.text().notNull(),
    payload_json: c.text().notNull().default("{}"),
    created_at: c.text().notNull().default(sql("CURRENT_TIMESTAMP")),
  }).index("idx_audit_actor", ["actor_id"]),
});
`, templateImportPath, templateName, emailCollation, minDisplayLen, minProjectNameLen, maxPriority, minTodoTitleLen, strings.Join(statusList, ","), maxPriority, optionalTodoCols, todoFTSColumns, extraTables)
}

func sanitizeName(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
			continue
		}
		b.WriteRune('-')
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "sim"
	}
	return out
}

func (c *client) query(preferOperation string, body map[string]any) (int, []byte, *apiError, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return 0, nil, nil, err
	}

	url := fmt.Sprintf("%s/data/query/%s", c.baseURL, c.table)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return 0, nil, nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Database", c.database)
	req.Header.Set("Prefer", "operation="+preferOperation)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return res.StatusCode, nil, nil, err
	}

	var e apiError
	if res.StatusCode >= 400 {
		_ = json.Unmarshal(raw, &e)
		return res.StatusCode, raw, &e, nil
	}

	return res.StatusCode, raw, nil, nil
}

func shouldFailStatus(cfg config, status int) bool {
	if status >= 500 {
		return true
	}
	if status >= 400 && cfg.failOn4xx {
		return true
	}
	return false
}

func weightedOp(r *rand.Rand, ops []op) op {
	v := r.Intn(100)
	switch {
	case v < 30:
		return ops[0] // insert
	case v < 50:
		return ops[1] // update
	case v < 70:
		return ops[2] // upsert
	case v < 85:
		return ops[3] // delete
	default:
		return ops[4] // select
	}
}

func randomTodo(r *rand.Rand) todo {
	return todo{
		ID:        r.Intn(200) + 1,
		Title:     fmt.Sprintf("sim-%d", r.Intn(1_000_000)),
		Completed: r.Intn(2) == 0,
	}
}

func pickKnownOrRandomID(r *rand.Rand, m map[int]todo) int {
	if len(m) > 0 && r.Intn(100) < 75 {
		i := r.Intn(len(m))
		j := 0
		for id := range m {
			if j == i {
				return id
			}
			j++
		}
	}
	return r.Intn(200) + 1
}

func todoToMap(cfg config, t todo) map[string]any {
	return map[string]any{
		cfg.idColumn:     t.ID,
		cfg.titleColumn:  t.Title,
		cfg.completedCol: t.Completed,
	}
}

func equalTodoMaps(a, b map[int]todo) bool {
	if len(a) != len(b) {
		return false
	}
	for id, x := range a {
		y, ok := b[id]
		if !ok {
			return false
		}
		if x.Title != y.Title || x.Completed != y.Completed {
			return false
		}
	}
	return true
}

func formatTodos(m map[int]todo) string {
	ids := make([]int, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		t := m[id]
		out = append(out, fmt.Sprintf("%d:{title=%q completed=%t}", id, t.Title, t.Completed))
	}
	return "[" + strings.Join(out, ", ") + "]"
}

func toInt(v any) (int, bool) {
	switch x := v.(type) {
	case float64:
		return int(x), true
	case int:
		return x, true
	case int64:
		return int(x), true
	case json.Number:
		n, err := x.Int64()
		if err != nil {
			return 0, false
		}
		return int(n), true
	default:
		return 0, false
	}
}

func toBool(v any) (bool, bool) {
	switch x := v.(type) {
	case bool:
		return x, true
	case float64:
		return x != 0, true
	case int:
		return x != 0, true
	default:
		return false, false
	}
}

func envOr(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func envIntOr(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envInt64Or(key string, fallback int64) int64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

func envBoolOr(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func printReplayHint(cfg config, seed int64) {
	fmt.Println("Replay:")
	fmt.Printf("go run . -base-url %q -database %q -table %q -seed %d -steps %d\n",
		cfg.baseURL, cfg.database, cfg.table, seed, cfg.steps)
}
