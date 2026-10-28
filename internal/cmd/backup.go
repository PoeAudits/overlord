package cmd

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/fileutil"
	"github.com/PoeAudits/overlord/internal/registry"
)

const backupTag = "backup"

var (
	backupLoadRegistry = loadRegistry
	backupSaveRegistry = saveRegistry
	backupNow          = time.Now
	backupOpts         backupOptions
)

type backupOptions struct {
	dryRun    bool
	listFiles bool
}

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Create a source backup zip for tagged projects",
	Long: `Create a timestamped backup zip in your home directory for projects tagged
with "backup" in the overlord registry.

For git repositories, backup includes tracked files plus untracked files that are
not ignored by .gitignore. Files matching .env* are always excluded.

Examples:
  overlord backup                  # Create ~/overlord-backup-YYYYMMDD-HHMMSS.zip
  overlord backup --dry-run        # Show project and file counts without writing
  overlord backup --dry-run --files # Show every file that would be included
  overlord backup list             # List projects in the backup set
  overlord backup add api web-app  # Add projects to the backup set
  overlord backup rm old-project   # Remove projects from the backup set`,
	Args: cobra.NoArgs,
	RunE: runBackup,
}

var backupAddCmd = &cobra.Command{
	Use:   "add <project> [project...]",
	Short: "Add projects to the backup set",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runBackupAdd,
}

var backupRmCmd = &cobra.Command{
	Use:     "rm <project> [project...]",
	Aliases: []string{"remove"},
	Short:   "Remove projects from the backup set",
	Args:    cobra.MinimumNArgs(1),
	RunE:    runBackupRm,
}

var backupListCmd = &cobra.Command{
	Use:   "list",
	Short: "Compare all projects against the backup set",
	Args:  cobra.NoArgs,
	RunE:  runBackupList,
}

func init() {
	rootCmd.AddCommand(backupCmd)
	backupCmd.AddCommand(backupAddCmd, backupRmCmd, backupListCmd)
	backupCmd.Flags().BoolVar(&backupOpts.dryRun, "dry-run", false, "Show what would be backed up without creating a zip")
	backupCmd.Flags().BoolVar(&backupOpts.listFiles, "files", false, "List files included in the backup output")
}

type backupProject struct {
	name string
	path string
}

type backupListProject struct {
	name       string
	path       string
	category   string
	lang       string
	status     string
	lastOpened string
	backedUp   bool
}

func runBackup(cmd *cobra.Command, args []string) error {
	reg, err := backupLoadRegistry()
	if err != nil {
		return err
	}

	projects, err := backupProjects(reg)
	if err != nil {
		return err
	}
	if len(projects) == 0 {
		return cmdError("backup", "no projects are tagged for backup", "Run 'overlord backup add <project>' first")
	}

	zipPath, rootName, err := backupOutputPath(backupNow())
	if err != nil {
		return err
	}
	projectFiles, totalFiles, err := collectBackupFiles(projects)
	if err != nil {
		return err
	}

	if backupOpts.dryRun {
		printBackupPlan(zipPath, rootName, projects, projectFiles, totalFiles)
		return nil
	}

	file, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("failed to create backup zip: %w", err)
	}

	zipWriter := zip.NewWriter(file)

	if err := writeBackupProjects(zipWriter, rootName, projects, projectFiles); err != nil {
		zipWriter.Close()
		file.Close()
		_ = os.Remove(zipPath)
		return err
	}

	if err := zipWriter.Close(); err != nil {
		file.Close()
		_ = os.Remove(zipPath)
		return fmt.Errorf("failed to finalize backup zip: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(zipPath)
		return fmt.Errorf("failed to close backup zip: %w", err)
	}

	fmt.Printf("Created backup: %s\n", fileutil.FormatPathWithTilde(zipPath))
	fmt.Printf("Projects: %d\n", len(projects))
	fmt.Printf("Files: %d\n", totalFiles)

	return nil
}

func writeBackupProjects(writer *zip.Writer, rootName string, projects []backupProject, files map[string][]string) error {
	for _, project := range projects {
		for _, relPath := range files[project.name] {
			sourcePath := filepath.Join(project.path, relPath)
			zipName := filepath.ToSlash(filepath.Join(rootName, safeZipSegment(project.name), relPath))
			if _, err := addPathToZip(writer, sourcePath, zipName); err != nil {
				return fmt.Errorf("failed to add %s: %w", sourcePath, err)
			}
		}
	}
	return nil
}

func collectBackupFiles(projects []backupProject) (map[string][]string, int, error) {
	projectFiles := make(map[string][]string, len(projects))
	totalFiles := 0
	for _, project := range projects {
		files, err := backupProjectFiles(project.path)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to collect files for '%s': %w", project.name, err)
		}
		projectFiles[project.name] = files
		totalFiles += len(files)
	}
	return projectFiles, totalFiles, nil
}

func printBackupPlan(zipPath, rootName string, projects []backupProject, projectFiles map[string][]string, totalFiles int) {
	fmt.Printf("Would create backup: %s\n", fileutil.FormatPathWithTilde(zipPath))
	fmt.Printf("Projects: %d\n", len(projects))
	fmt.Printf("Files: %d\n", totalFiles)
	for _, project := range projects {
		fmt.Printf("%s\t%d files\n", project.name, len(projectFiles[project.name]))
	}
	if !backupOpts.listFiles {
		return
	}
	for _, project := range projects {
		for _, relPath := range projectFiles[project.name] {
			fmt.Println(filepath.ToSlash(filepath.Join(rootName, safeZipSegment(project.name), relPath)))
		}
	}
}

func runBackupAdd(cmd *cobra.Command, args []string) error {
	return updateBackupTags(args, true)
}

func runBackupRm(cmd *cobra.Command, args []string) error {
	return updateBackupTags(args, false)
}

func runBackupList(cmd *cobra.Command, args []string) error {
	reg, err := backupLoadRegistry()
	if err != nil {
		return err
	}

	projects := backupListProjects(reg)
	backedUp, notBackedUp := splitBackupListProjects(projects)

	styles := NewStyles()
	printBackupListSection("IN BACKUP", backedUp, styles.Success, styles)
	printBackupListSection("NOT BACKED UP", notBackedUp, styles.StrongWarn, styles)

	return nil
}

func backupListProjects(reg *registry.Registry) []backupListProject {
	projects := make([]backupListProject, 0, len(reg.Projects))
	for name, project := range reg.Projects {
		projectPath, err := fileutil.ExpandProjectPath(project.Path, reg.Settings.BaseDir)
		if err != nil {
			projectPath = project.Path
		}
		projects = append(projects, backupListProject{
			name:       name,
			path:       projectPath,
			category:   string(project.Category),
			lang:       string(project.Lang),
			status:     string(project.Status.State),
			lastOpened: project.LastOpened,
			backedUp:   hasBackupTag(project),
		})
	}
	sortBackupListProjects(projects)
	return projects
}

func splitBackupListProjects(projects []backupListProject) ([]backupListProject, []backupListProject) {
	backedUp := make([]backupListProject, 0, len(projects))
	notBackedUp := make([]backupListProject, 0, len(projects))
	for _, project := range projects {
		if project.backedUp {
			backedUp = append(backedUp, project)
		} else {
			notBackedUp = append(notBackedUp, project)
		}
	}
	return backedUp, notBackedUp
}

func sortBackupListProjects(projects []backupListProject) {
	sort.Slice(projects, func(i, j int) bool {
		if projects[i].status != projects[j].status {
			return statusRank(projects[i].status) < statusRank(projects[j].status)
		}
		if projects[i].lastOpened != projects[j].lastOpened {
			if projects[i].lastOpened == "" {
				return false
			}
			if projects[j].lastOpened == "" {
				return true
			}
			return projects[i].lastOpened > projects[j].lastOpened
		}
		if projects[i].category != projects[j].category {
			return projects[i].category < projects[j].category
		}
		if projects[i].lang != projects[j].lang {
			return projects[i].lang < projects[j].lang
		}
		return projects[i].name < projects[j].name
	})
}

func printBackupListSection(title string, projects []backupListProject, titleStyle lipgloss.Style, styles styles) {
	fmt.Println(titleStyle.Render(fmt.Sprintf("%s (%d)", title, len(projects))))
	if len(projects) == 0 {
		fmt.Println(styles.TableMuted.Render("  none"))
		fmt.Println()
		return
	}

	rows := backupListTableRows(projects)
	t := table.New().
		Border(lipgloss.HiddenBorder()).
		StyleFunc(backupListTableStyleFunc(rows, styles)).
		Headers("NAME", "CATEGORY", "LANG", "OPENED", "STATUS", "PATH").
		Rows(rows...)
	fmt.Println(t)
	fmt.Println()
}

func backupListTableRows(projects []backupListProject) [][]string {
	rows := make([][]string, 0, len(projects))
	for _, project := range projects {
		rows = append(rows, []string{
			project.name,
			project.category,
			project.lang,
			relativeTime(project.lastOpened),
			project.status,
			fileutil.FormatPathWithTilde(project.path),
		})
	}
	return rows
}

func backupListTableStyleFunc(rows [][]string, styles styles) func(row, col int) lipgloss.Style {
	return func(row, col int) lipgloss.Style {
		if row == table.HeaderRow {
			return styles.TableHeader
		}
		if row < 0 || row >= len(rows) {
			return styles.TableCell
		}

		switch col {
		case 0:
			return styles.TableName
		case 1:
			return listColorStyle(styles.TableCell, rows[row][1], categoryColors, styles.TableMuted)
		case 2:
			return listColorStyle(styles.TableCell, rows[row][2], languageColors, styles.TableMuted)
		case 3:
			return styles.TableRecent
		case 4:
			return listStatusStyle(rows[row][4], false, styles)
		case 5:
			return styles.TableMuted
		default:
			return styles.TableCell
		}
	}
}

func updateBackupTags(args []string, add bool) error {
	reg, err := backupLoadRegistry()
	if err != nil {
		return err
	}

	update := removeBackupTag
	changedMessage, unchangedMessage := "Removed from backup", "Not in backup"
	if add {
		update = addBackupTag
		changedMessage, unchangedMessage = "Added to backup", "Already in backup"
	}

	changed := false
	seen := make(map[string]struct{}, len(args))
	for _, arg := range args {
		result, err := reg.ResolveOne(arg)
		if err != nil {
			return err
		}
		if _, ok := seen[result.Name]; ok {
			continue
		}
		seen[result.Name] = struct{}{}

		project := result.Project
		message := unchangedMessage
		if update(&project) {
			changed = true
			message = changedMessage
		}
		fmt.Printf("%s: %s\n", message, result.Name)
		reg.Projects[result.Name] = project
	}

	if changed {
		return backupSaveRegistry(reg)
	}
	return nil
}

func backupProjects(reg *registry.Registry) ([]backupProject, error) {
	names := make([]string, 0, len(reg.Projects))
	for name, project := range reg.Projects {
		if hasBackupTag(project) {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	projects := make([]backupProject, 0, len(names))
	for _, name := range names {
		project := reg.Projects[name]
		projectPath, err := fileutil.ExpandProjectPath(project.Path, reg.Settings.BaseDir)
		if err != nil {
			return nil, fmt.Errorf("failed to expand path for '%s': %w", name, err)
		}
		info, err := os.Stat(projectPath)
		if err != nil {
			return nil, fmt.Errorf("project '%s' path is unavailable: %w", name, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("project '%s' path is not a directory: %s", name, projectPath)
		}
		projects = append(projects, backupProject{name: name, path: projectPath})
	}

	return projects, nil
}

func backupOutputPath(now time.Time) (string, string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve home directory: %w", err)
	}

	rootName := "overlord-backup-" + now.Format("20060102-150405")
	return filepath.Join(homeDir, rootName+".zip"), rootName, nil
}

func backupProjectFiles(projectPath string) ([]string, error) {
	files, usedGit, err := gitBackupFiles(projectPath)
	if usedGit || err != nil {
		return files, err
	}
	return fallbackBackupFiles(projectPath)
}

func gitBackupFiles(projectPath string) ([]string, bool, error) {
	if err := exec.Command("git", "-C", projectPath, "rev-parse", "--is-inside-work-tree").Run(); err != nil {
		return nil, false, nil
	}

	cmd := exec.Command("git", "-C", projectPath, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", ".")
	output, err := cmd.Output()
	if err != nil {
		return nil, true, fmt.Errorf("git ls-files failed: %w", err)
	}

	files := cleanBackupFileList(bytes.Split(output, []byte{0}))
	files, err = filterExistingBackupFiles(projectPath, files)
	return files, true, err
}

func fallbackBackupFiles(projectPath string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(projectPath, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(projectPath, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		if entry.IsDir() {
			if shouldExcludeBackupPath(relPath) || shouldSkipFallbackDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		if shouldExcludeBackupPath(relPath) || shouldSkipFallbackFile(entry.Name()) {
			return nil
		}
		files = append(files, relPath)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}

func filterExistingBackupFiles(projectPath string, files []string) ([]string, error) {
	filtered := files[:0]
	for _, relPath := range files {
		info, err := os.Lstat(filepath.Join(projectPath, relPath))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if info.IsDir() || (info.Mode()&os.ModeSymlink == 0 && !info.Mode().IsRegular()) {
			continue
		}
		filtered = append(filtered, relPath)
	}
	return filtered, nil
}

func cleanBackupFileList(entries [][]byte) []string {
	seen := make(map[string]struct{}, len(entries))
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if len(entry) == 0 {
			continue
		}
		relPath := filepath.Clean(string(entry))
		if relPath == "." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || filepath.IsAbs(relPath) {
			continue
		}
		if shouldExcludeBackupPath(relPath) {
			continue
		}
		if _, ok := seen[relPath]; ok {
			continue
		}
		seen[relPath] = struct{}{}
		files = append(files, relPath)
	}
	sort.Strings(files)
	return files
}

func addPathToZip(zipWriter *zip.Writer, sourcePath, zipName string) (bool, error) {
	info, err := os.Lstat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if info.IsDir() {
		return false, nil
	}
	if info.Mode()&os.ModeSymlink == 0 && !info.Mode().IsRegular() {
		return false, nil
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return false, err
	}
	header.Name = zipName
	header.Method = zip.Deflate
	header.SetMode(info.Mode())

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return false, err
	}

	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(sourcePath)
		if err != nil {
			return false, err
		}
		_, err = writer.Write([]byte(target))
		return err == nil, err
	}

	file, err := os.Open(sourcePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	_, err = io.Copy(writer, file)
	return err == nil, err
}

func hasBackupTag(project registry.Project) bool {
	for _, tag := range project.Tags {
		if tag == backupTag {
			return true
		}
	}
	return false
}

func addBackupTag(project *registry.Project) bool {
	if hasBackupTag(*project) {
		return false
	}
	project.Tags = append(project.Tags, backupTag)
	sort.Strings(project.Tags)
	return true
}

func removeBackupTag(project *registry.Project) bool {
	removed := false
	tags := project.Tags[:0]
	for _, tag := range project.Tags {
		if tag == backupTag {
			removed = true
			continue
		}
		tags = append(tags, tag)
	}
	project.Tags = tags
	return removed
}

func shouldExcludeBackupPath(relPath string) bool {
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	if len(parts) > 0 && shouldSkipBackupTopLevelDir(parts[0]) {
		return true
	}
	for _, part := range parts {
		if shouldSkipBackupAnyDir(part) {
			return true
		}
		if matched, _ := filepath.Match(".env*", part); matched {
			return true
		}
	}
	if shouldExcludeBackupFile(filepath.Base(relPath)) {
		return true
	}
	return false
}

func shouldSkipBackupAnyDir(name string) bool {
	skipped := map[string]struct{}{
		".venv":        {},
		"__pycache__":  {},
		"env":          {},
		"node_modules": {},
		"target":       {},
		"venv":         {},
	}
	_, ok := skipped[strings.ToLower(name)]
	return ok
}

func shouldExcludeBackupFile(name string) bool {
	patterns := []string{
		"*.log",
		"*.pyc",
		"*.pyo",
		"*.session",
		"*.session-journal",
		"*.tmp",
		"*.tsbuildinfo",
	}
	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
	}
	return name == ".DS_Store"
}

func shouldSkipBackupTopLevelDir(name string) bool {
	skipped := map[string]struct{}{
		".cache":        {},
		".git":          {},
		".mypy_cache":   {},
		".next":         {},
		".pytest_cache": {},
		".ruff_cache":   {},
		".turbo":        {},
		".venv":         {},
		".worktrees":    {},
		"artifacts":     {},
		"bin":           {},
		"broadcast":     {},
		"build":         {},
		"cache":         {},
		"coverage":      {},
		"dist":          {},
		"env":           {},
		"lib":           {},
		"logs":          {},
		"myenv":         {},
		"node_modules":  {},
		"out":           {},
		"target":        {},
		"tmp":           {},
		"venv":          {},
		"worktrees":     {},
	}
	_, ok := skipped[strings.ToLower(name)]
	return ok
}

func shouldSkipFallbackDir(name string) bool {
	skipped := map[string]struct{}{
		".cache":        {},
		".mypy_cache":   {},
		".next":         {},
		".pytest_cache": {},
		".ruff_cache":   {},
		".turbo":        {},
		".venv":         {},
		"__pycache__":   {},
		"node_modules":  {},
		"target":        {},
	}
	_, ok := skipped[name]
	return ok
}

func shouldSkipFallbackFile(name string) bool {
	return shouldExcludeBackupFile(name)
}

func safeZipSegment(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return "project"
	}
	return name
}
