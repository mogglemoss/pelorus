package fileinfo

import "testing"

// resolutionCases exercises the precedence order of Icon:
//   symlink > dir > filename > extension > exec-bit > default.
func TestIconResolutionOrder(t *testing.T) {
	cases := []struct {
		name string
		fi   FileInfo
		want iconKey
	}{
		// Symlink beats everything.
		{"broken symlink over dir", FileInfo{IsSymlink: true, SymlinkBroken: true, IsDir: true}, kSymlinkBroken},
		{"valid symlink", FileInfo{IsSymlink: true}, kSymlink},

		// Directory beats filename/extension.
		{"directory named dockerfile", FileInfo{Name: "Dockerfile", IsDir: true}, kDir},

		// Exact filename beats extension.
		{"Dockerfile (case-insensitive)", FileInfo{Name: "dockerfile"}, kDocker},
		{"Makefile", FileInfo{Name: "Makefile"}, kBuildManifest},
		{"go.mod", FileInfo{Name: "go.mod"}, kBuildManifest},
		{"go.sum is a lock", FileInfo{Name: "go.sum"}, kLock},
		{".gitignore", FileInfo{Name: ".gitignore"}, kGit},
		{"README no ext", FileInfo{Name: "README"}, kMarkdown},
		{"README.txt still markdown by prefix rule", FileInfo{Name: "README.txt"}, kMarkdown},

		// Extensions — code families.
		{"Go source", FileInfo{Name: "main.go"}, kCodeCompiled},
		{"Python source", FileInfo{Name: "script.py"}, kCodeScripting},
		{"TSX source", FileInfo{Name: "App.tsx"}, kCodeWeb},
		{"shell script", FileInfo{Name: "run.sh"}, kShell},

		// Extensions — specific fixes.
		{"PDF no longer collides with markdown", FileInfo{Name: "doc.pdf"}, kPDF},
		{"Markdown stays ≡", FileInfo{Name: "notes.md"}, kMarkdown},
		{"SQL goes to database", FileInfo{Name: "schema.sql"}, kDatabase},
		{"patch file", FileInfo{Name: "fix.patch"}, kPatch},
		{"font", FileInfo{Name: "Inter.ttf"}, kFont},
		{"iso disk image", FileInfo{Name: "ubuntu.iso"}, kDiskImage},
		{"log file", FileInfo{Name: "app.log"}, kLog},
		{"csv spreadsheet", FileInfo{Name: "data.csv"}, kSpreadsheet},
		{"docx", FileInfo{Name: "report.docx"}, kDocument},

		// Exec bit fallback — regular file, no known extension, mode +x.
		{"unix executable no ext", FileInfo{Name: "pelorus", Mode: 0o755}, kExec},
		{"unix executable with exec-ext still wins as exec", FileInfo{Name: "tool.bin", Mode: 0o755}, kExec},
		{"regular file no ext no exec bit", FileInfo{Name: "mystery", Mode: 0o644}, kDefault},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveIconKey(tc.fi)
			if got != tc.want {
				t.Fatalf("resolveIconKey(%+v) = %v, want %v", tc.fi, got, tc.want)
			}
		})
	}
}

// TestIconStyleDispatch confirms SetIconStyle switches the glyph table.
func TestIconStyleDispatch(t *testing.T) {
	fi := FileInfo{Name: "a.md"}

	SetIconStyle(IconStyleUnicode)
	if g := Icon(fi); g != unicodeIcons[kMarkdown] {
		t.Fatalf("unicode markdown: got %q want %q", g, unicodeIcons[kMarkdown])
	}
	SetIconStyle(IconStyleNerd)
	if g := Icon(fi); g != nerdIcons[kMarkdown] {
		t.Fatalf("nerd markdown: got %q want %q", g, nerdIcons[kMarkdown])
	}
	// Restore default so other tests aren't affected.
	SetIconStyle(IconStyleUnicode)
}

// TestIconTablesComplete ensures both glyph tables cover every key.
// Catches the case where adding a new iconKey drops a nerd mapping.
func TestIconTablesComplete(t *testing.T) {
	for k := kDefault; k <= kText; k++ {
		if _, ok := unicodeIcons[k]; !ok {
			t.Errorf("unicodeIcons missing key %d", k)
		}
		if _, ok := nerdIcons[k]; !ok {
			t.Errorf("nerdIcons missing key %d", k)
		}
	}
}

