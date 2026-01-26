package sync

import (
	"errors"
	"strings"
	"testing"
)

// MockExecutor is a mock implementation of CommandExecutor for testing
type MockExecutor struct {
	// ExpectedName is the expected command name
	ExpectedName string
	// ExpectedArgs is the expected arguments (if set, will be validated)
	ExpectedArgs []string
	// Output is the output to return
	Output []byte
	// ExitCode is the exit code to return
	ExitCode int
	// Err is the error to return (for command execution failures)
	Err error
	// RecordedName stores the actual command name called
	RecordedName string
	// RecordedArgs stores the actual arguments called
	RecordedArgs []string
}

func (m *MockExecutor) Execute(name string, args ...string) ([]byte, int, error) {
	m.RecordedName = name
	m.RecordedArgs = args

	if m.Err != nil {
		return nil, -1, m.Err
	}

	return m.Output, m.ExitCode, nil
}

func TestRsyncOptions_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		opts    RsyncOptions
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid options",
			opts: RsyncOptions{
				Source: "/local/path",
				Dest:   "host:/remote/path",
			},
			wantErr: false,
		},
		{
			name: "valid options with excludes",
			opts: RsyncOptions{
				Source:   "/local/path",
				Dest:     "host:/remote/path",
				Excludes: []string{"node_modules", ".git"},
			},
			wantErr: false,
		},
		{
			name: "missing source",
			opts: RsyncOptions{
				Dest: "host:/remote/path",
			},
			wantErr: true,
			errMsg:  "source path is required",
		},
		{
			name: "missing destination",
			opts: RsyncOptions{
				Source: "/local/path",
			},
			wantErr: true,
			errMsg:  "destination path is required",
		},
		{
			name:    "empty options",
			opts:    RsyncOptions{},
			wantErr: true,
			errMsg:  "source path is required",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("Validate() error = %v, want error containing %q", err, tt.errMsg)
			}
		})
	}
}

func TestRsync_buildArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		opts     RsyncOptions
		wantArgs []string
	}{
		{
			name: "basic sync",
			opts: RsyncOptions{
				Source: "/local/path",
				Dest:   "host:/remote/path",
			},
			wantArgs: []string{"-avz", "--progress", "/local/path/", "host:/remote/path"},
		},
		{
			name: "with trailing slash on source",
			opts: RsyncOptions{
				Source: "/local/path/",
				Dest:   "host:/remote/path",
			},
			wantArgs: []string{"-avz", "--progress", "/local/path/", "host:/remote/path"},
		},
		{
			name: "with dry-run",
			opts: RsyncOptions{
				Source: "/local/path",
				Dest:   "host:/remote/path",
				DryRun: true,
			},
			wantArgs: []string{"-avz", "--progress", "--dry-run", "/local/path/", "host:/remote/path"},
		},
		{
			name: "with delete",
			opts: RsyncOptions{
				Source: "/local/path",
				Dest:   "host:/remote/path",
				Delete: true,
			},
			wantArgs: []string{"-avz", "--progress", "--delete", "/local/path/", "host:/remote/path"},
		},
		{
			name: "with excludes",
			opts: RsyncOptions{
				Source:   "/local/path",
				Dest:     "host:/remote/path",
				Excludes: []string{"node_modules", ".git", "*.log"},
			},
			wantArgs: []string{"-avz", "--progress", "--exclude=node_modules", "--exclude=.git", "--exclude=*.log", "/local/path/", "host:/remote/path"},
		},
		{
			name: "with all options",
			opts: RsyncOptions{
				Source:   "/local/path",
				Dest:     "host:/remote/path",
				Excludes: []string{"node_modules"},
				DryRun:   true,
				Delete:   true,
			},
			wantArgs: []string{"-avz", "--progress", "--dry-run", "--delete", "--exclude=node_modules", "/local/path/", "host:/remote/path"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := NewRsync()
			args := r.buildArgs(tt.opts)

			if len(args) != len(tt.wantArgs) {
				t.Errorf("buildArgs() got %d args, want %d args\ngot:  %v\nwant: %v", len(args), len(tt.wantArgs), args, tt.wantArgs)
				return
			}

			for i, arg := range args {
				if arg != tt.wantArgs[i] {
					t.Errorf("buildArgs()[%d] = %q, want %q", i, arg, tt.wantArgs[i])
				}
			}
		})
	}
}

func TestRsync_Push(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		opts       RsyncOptions
		mockOutput []byte
		mockExit   int
		mockErr    error
		wantErr    bool
		errMsg     string
	}{
		{
			name: "successful push",
			opts: RsyncOptions{
				Source: "/local/path",
				Dest:   "host:/remote/path",
			},
			mockOutput: []byte("sending incremental file list\n\nsent 100 bytes  received 50 bytes\n"),
			mockExit:   0,
			wantErr:    false,
		},
		{
			name: "push with excludes",
			opts: RsyncOptions{
				Source:   "/local/path",
				Dest:     "host:/remote/path",
				Excludes: []string{"node_modules", ".git"},
			},
			mockOutput: []byte("sending incremental file list\n"),
			mockExit:   0,
			wantErr:    false,
		},
		{
			name: "push with non-zero exit code",
			opts: RsyncOptions{
				Source: "/local/path",
				Dest:   "host:/remote/path",
			},
			mockOutput: []byte("rsync: connection unexpectedly closed\n"),
			mockExit:   12,
			wantErr:    true,
			errMsg:     "rsync failed with exit code 12",
		},
		{
			name: "push with invalid options",
			opts: RsyncOptions{
				Source: "",
				Dest:   "host:/remote/path",
			},
			wantErr: true,
			errMsg:  "source path is required",
		},
		{
			name: "push with executor error",
			opts: RsyncOptions{
				Source: "/local/path",
				Dest:   "host:/remote/path",
			},
			mockErr: errors.New("command not found"),
			wantErr: true,
			errMsg:  "command not found",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &MockExecutor{
				Output:   tt.mockOutput,
				ExitCode: tt.mockExit,
				Err:      tt.mockErr,
			}
			r := NewRsyncWithExecutor(mock)

			result, err := r.Push(tt.opts)

			if (err != nil) != tt.wantErr {
				t.Errorf("Push() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("Push() error = %v, want error containing %q", err, tt.errMsg)
			}

			if !tt.wantErr && result == nil {
				t.Error("Push() returned nil result on success")
			}

			if !tt.wantErr && result != nil {
				if result.ExitCode != tt.mockExit {
					t.Errorf("Push() result.ExitCode = %d, want %d", result.ExitCode, tt.mockExit)
				}
				if result.Output != string(tt.mockOutput) {
					t.Errorf("Push() result.Output = %q, want %q", result.Output, string(tt.mockOutput))
				}
			}

			// Verify rsync was called with correct command
			if tt.mockErr == nil && tt.opts.Source != "" && tt.opts.Dest != "" {
				if mock.RecordedName != "rsync" {
					t.Errorf("Push() called command %q, want %q", mock.RecordedName, "rsync")
				}
			}
		})
	}
}

func TestRsync_Pull(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		opts       RsyncOptions
		mockOutput []byte
		mockExit   int
		mockErr    error
		wantErr    bool
		errMsg     string
	}{
		{
			name: "successful pull",
			opts: RsyncOptions{
				Source: "host:/remote/path",
				Dest:   "/local/path",
			},
			mockOutput: []byte("receiving incremental file list\n\nreceived 100 bytes  sent 50 bytes\n"),
			mockExit:   0,
			wantErr:    false,
		},
		{
			name: "pull with excludes",
			opts: RsyncOptions{
				Source:   "host:/remote/path",
				Dest:     "/local/path",
				Excludes: []string{"*.tmp", "cache/"},
			},
			mockOutput: []byte("receiving incremental file list\n"),
			mockExit:   0,
			wantErr:    false,
		},
		{
			name: "pull with non-zero exit code",
			opts: RsyncOptions{
				Source: "host:/remote/path",
				Dest:   "/local/path",
			},
			mockOutput: []byte("rsync: connection refused\n"),
			mockExit:   10,
			wantErr:    true,
			errMsg:     "rsync failed with exit code 10",
		},
		{
			name: "pull with invalid options",
			opts: RsyncOptions{
				Source: "host:/remote/path",
				Dest:   "",
			},
			wantErr: true,
			errMsg:  "destination path is required",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &MockExecutor{
				Output:   tt.mockOutput,
				ExitCode: tt.mockExit,
				Err:      tt.mockErr,
			}
			r := NewRsyncWithExecutor(mock)

			result, err := r.Pull(tt.opts)

			if (err != nil) != tt.wantErr {
				t.Errorf("Pull() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("Pull() error = %v, want error containing %q", err, tt.errMsg)
			}

			if !tt.wantErr && result == nil {
				t.Error("Pull() returned nil result on success")
			}

			if !tt.wantErr && result != nil {
				if result.ExitCode != tt.mockExit {
					t.Errorf("Pull() result.ExitCode = %d, want %d", result.ExitCode, tt.mockExit)
				}
			}
		})
	}
}

func TestRsync_DryRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		opts       RsyncOptions
		mockOutput []byte
		mockExit   int
		mockErr    error
		wantErr    bool
		errMsg     string
	}{
		{
			name: "successful dry-run",
			opts: RsyncOptions{
				Source: "/local/path",
				Dest:   "host:/remote/path",
			},
			mockOutput: []byte("sending incremental file list\nfile1.txt\nfile2.txt\n\nsent 100 bytes  received 50 bytes (dry run)\n"),
			mockExit:   0,
			wantErr:    false,
		},
		{
			name: "dry-run with delete",
			opts: RsyncOptions{
				Source: "/local/path",
				Dest:   "host:/remote/path",
				Delete: true,
			},
			mockOutput: []byte("sending incremental file list\ndeleting old-file.txt\n"),
			mockExit:   0,
			wantErr:    false,
		},
		{
			name: "dry-run with non-zero exit code",
			opts: RsyncOptions{
				Source: "/local/path",
				Dest:   "host:/remote/path",
			},
			mockOutput: []byte("rsync error: some error\n"),
			mockExit:   1,
			wantErr:    true,
			errMsg:     "rsync dry-run failed with exit code 1",
		},
		{
			name: "dry-run with invalid options",
			opts: RsyncOptions{
				Source: "",
				Dest:   "",
			},
			wantErr: true,
			errMsg:  "source path is required",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &MockExecutor{
				Output:   tt.mockOutput,
				ExitCode: tt.mockExit,
				Err:      tt.mockErr,
			}
			r := NewRsyncWithExecutor(mock)

			output, err := r.DryRun(tt.opts)

			if (err != nil) != tt.wantErr {
				t.Errorf("DryRun() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("DryRun() error = %v, want error containing %q", err, tt.errMsg)
			}

			if !tt.wantErr && output != string(tt.mockOutput) {
				t.Errorf("DryRun() output = %q, want %q", output, string(tt.mockOutput))
			}

			// Verify --dry-run flag is always included
			if tt.mockErr == nil && tt.opts.Source != "" && tt.opts.Dest != "" {
				foundDryRun := false
				for _, arg := range mock.RecordedArgs {
					if arg == "--dry-run" {
						foundDryRun = true
						break
					}
				}
				if !foundDryRun {
					t.Error("DryRun() did not include --dry-run flag in arguments")
				}
			}
		})
	}
}

func TestFormatRemotePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		host string
		path string
		want string
	}{
		{
			name: "simple host and path",
			host: "myhost",
			path: "/home/user/data",
			want: "myhost:/home/user/data",
		},
		{
			name: "hostname with domain",
			host: "server.example.com",
			path: "/var/www",
			want: "server.example.com:/var/www",
		},
		{
			name: "relative path",
			host: "backup-server",
			path: "backups/daily",
			want: "backup-server:backups/daily",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := FormatRemotePath(tt.host, tt.path)
			if got != tt.want {
				t.Errorf("FormatRemotePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatRemotePathWithUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		user string
		host string
		path string
		want string
	}{
		{
			name: "user, host, and path",
			user: "admin",
			host: "myhost",
			path: "/home/admin/data",
			want: "admin@myhost:/home/admin/data",
		},
		{
			name: "root user",
			user: "root",
			host: "server.example.com",
			path: "/etc/config",
			want: "root@server.example.com:/etc/config",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := FormatRemotePathWithUser(tt.user, tt.host, tt.path)
			if got != tt.want {
				t.Errorf("FormatRemotePathWithUser() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseRemotePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		remotePath string
		wantUser   string
		wantHost   string
		wantPath   string
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "host and path",
			remotePath: "myhost:/home/user/data",
			wantUser:   "",
			wantHost:   "myhost",
			wantPath:   "/home/user/data",
			wantErr:    false,
		},
		{
			name:       "user, host, and path",
			remotePath: "admin@myhost:/home/admin/data",
			wantUser:   "admin",
			wantHost:   "myhost",
			wantPath:   "/home/admin/data",
			wantErr:    false,
		},
		{
			name:       "hostname with domain",
			remotePath: "user@server.example.com:/var/www",
			wantUser:   "user",
			wantHost:   "server.example.com",
			wantPath:   "/var/www",
			wantErr:    false,
		},
		{
			name:       "relative path",
			remotePath: "backup-server:backups/daily",
			wantUser:   "",
			wantHost:   "backup-server",
			wantPath:   "backups/daily",
			wantErr:    false,
		},
		{
			name:       "empty path",
			remotePath: "host:",
			wantUser:   "",
			wantHost:   "host",
			wantPath:   "",
			wantErr:    false,
		},
		{
			name:       "missing colon",
			remotePath: "myhost/home/user/data",
			wantErr:    true,
			errMsg:     "missing colon separator",
		},
		{
			name:       "empty host",
			remotePath: ":/path",
			wantErr:    true,
			errMsg:     "empty host",
		},
		{
			name:       "empty host with user",
			remotePath: "user@:/path",
			wantErr:    true,
			errMsg:     "empty host",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			user, host, path, err := ParseRemotePath(tt.remotePath)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRemotePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ParseRemotePath() error = %v, want error containing %q", err, tt.errMsg)
				return
			}

			if !tt.wantErr {
				if user != tt.wantUser {
					t.Errorf("ParseRemotePath() user = %q, want %q", user, tt.wantUser)
				}
				if host != tt.wantHost {
					t.Errorf("ParseRemotePath() host = %q, want %q", host, tt.wantHost)
				}
				if path != tt.wantPath {
					t.Errorf("ParseRemotePath() path = %q, want %q", path, tt.wantPath)
				}
			}
		})
	}
}

func TestIsRemotePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "remote path with host",
			path: "myhost:/home/user/data",
			want: true,
		},
		{
			name: "remote path with user and host",
			path: "user@myhost:/home/user/data",
			want: true,
		},
		{
			name: "local absolute path",
			path: "/home/user/data",
			want: false,
		},
		{
			name: "local relative path",
			path: "data/files",
			want: false,
		},
		{
			name: "local path with colon in name",
			path: "/home/user/file:with:colons",
			want: false,
		},
		{
			name: "empty path",
			path: "",
			want: false,
		},
		{
			name: "windows drive letter",
			path: "C:/Users/data",
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := IsRemotePath(tt.path)
			if got != tt.want {
				t.Errorf("IsRemotePath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestNewRsync(t *testing.T) {
	t.Parallel()

	r := NewRsync()
	if r == nil {
		t.Fatal("NewRsync() returned nil")
	}
	if r.executor == nil {
		t.Error("NewRsync() executor is nil")
	}
}

func TestNewRsyncWithExecutor(t *testing.T) {
	t.Parallel()

	mock := &MockExecutor{}
	r := NewRsyncWithExecutor(mock)

	if r == nil {
		t.Fatal("NewRsyncWithExecutor() returned nil")
	}
	if r.executor != mock {
		t.Error("NewRsyncWithExecutor() did not set the provided executor")
	}
}

func TestRsync_Push_VerifiesArguments(t *testing.T) {
	t.Parallel()

	mock := &MockExecutor{
		Output:   []byte("success\n"),
		ExitCode: 0,
	}
	r := NewRsyncWithExecutor(mock)

	opts := RsyncOptions{
		Source:   "/local/path",
		Dest:     "host:/remote/path",
		Excludes: []string{"node_modules", ".git"},
		DryRun:   true,
		Delete:   true,
	}

	_, err := r.Push(opts)
	if err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	// Verify command name
	if mock.RecordedName != "rsync" {
		t.Errorf("Push() called %q, want %q", mock.RecordedName, "rsync")
	}

	// Verify all expected arguments are present
	expectedArgs := map[string]bool{
		"-avz":                   false,
		"--progress":             false,
		"--dry-run":              false,
		"--delete":               false,
		"--exclude=node_modules": false,
		"--exclude=.git":         false,
		"/local/path/":           false,
		"host:/remote/path":      false,
	}

	for _, arg := range mock.RecordedArgs {
		if _, ok := expectedArgs[arg]; ok {
			expectedArgs[arg] = true
		}
	}

	for arg, found := range expectedArgs {
		if !found {
			t.Errorf("Push() missing expected argument %q", arg)
		}
	}
}
