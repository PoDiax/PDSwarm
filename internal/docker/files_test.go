package docker_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/podiax/pdswarm/internal/docker"
)

func TestFiles_ValidationAndNilService(t *testing.T) {
	var nilSvc *docker.Service
	ctx := context.Background()

	// Nil service checks
	if _, err := nilSvc.StatContainerPath(ctx, "any", "/etc"); err == nil {
		t.Error("expected error calling StatContainerPath on nil service")
	}
	if _, err := nilSvc.ListContainerFiles(ctx, "any", "/etc"); err == nil {
		t.Error("expected error calling ListContainerFiles on nil service")
	}
	if _, _, err := nilSvc.ReadContainerFile(ctx, "any", "/etc/hosts", 100); err == nil {
		t.Error("expected error calling ReadContainerFile on nil service")
	}
	if _, _, err := nilSvc.DownloadContainerFile(ctx, "any", "/etc/hosts"); err == nil {
		t.Error("expected error calling DownloadContainerFile on nil service")
	}
	if err := nilSvc.UploadContainerFile(ctx, "any", "/tmp", "test.txt", strings.NewReader("hi"), 2, 0644); err == nil {
		t.Error("expected error calling UploadContainerFile on nil service")
	}
	if err := nilSvc.DeleteContainerFile(ctx, "any", "/tmp/test.txt"); err == nil {
		t.Error("expected error calling DeleteContainerFile on nil service")
	}
}

func TestFiles_DeleteRootSafety(t *testing.T) {
	svc, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create docker service: %v", err)
	}
	defer svc.Close()

	ctx := context.Background()
	// Protection against deleting root or empty paths
	for _, p := range []string{"/", "", ".", "///"} {
		err := svc.DeleteContainerFile(ctx, "any-id", p)
		if err == nil || !strings.Contains(err.Error(), "refusing to delete") {
			t.Errorf("expected safety refusal when deleting %q, got: %v", p, err)
		}
	}
}

func TestFiles_LiveContainerOperations(t *testing.T) {
	svc, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create docker service: %v", err)
	}
	defer svc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := svc.Ping(ctx); err != nil {
		t.Skipf("Docker daemon not accessible: %v", err)
	}

	containers, err := svc.ListContainers(ctx, false)
	if err != nil || len(containers) == 0 {
		t.Skip("No running containers found for live files test")
	}

	containerID := containers[0].ID
	t.Logf("Testing files operations on container %s (%s)", containerID[:12], containers[0].Image)

	// 1. List files on /
	rootFiles, err := svc.ListContainerFiles(ctx, containerID, "/")
	if err != nil {
		t.Fatalf("failed to list / in container: %v", err)
	}
	if len(rootFiles) == 0 {
		t.Fatal("expected non-empty root directory listing")
	}

	// Verify sorting: directories first
	seenNonDir := false
	for _, fi := range rootFiles {
		if !fi.IsDir {
			seenNonDir = true
		} else if seenNonDir {
			t.Errorf("sorting violation: directory %s appeared after non-directory", fi.Name)
		}
	}

	// 2. Upload a test file to /tmp
	testFilename := "pdswarm_test_file.txt"
	testContent := "Hello from PDSwarm Container File Browser Backend!\n"
	err = svc.UploadContainerFile(ctx, containerID, "/tmp", testFilename, strings.NewReader(testContent), int64(len(testContent)), 0644)
	if err != nil {
		t.Fatalf("failed to upload file: %v", err)
	}

	testFilePath := "/tmp/" + testFilename

	// 3. Stat the uploaded file
	stat, err := svc.StatContainerPath(ctx, containerID, testFilePath)
	if err != nil {
		t.Fatalf("failed to stat uploaded file %s: %v", testFilePath, err)
	}
	if stat.Size != int64(len(testContent)) {
		t.Errorf("expected size %d, got %d", len(testContent), stat.Size)
	}

	// 4. List /tmp and verify the file is present
	tmpFiles, err := svc.ListContainerFiles(ctx, containerID, "/tmp")
	if err != nil {
		t.Fatalf("failed to list /tmp: %v", err)
	}
	var found bool
	for _, fi := range tmpFiles {
		if fi.Name == testFilename {
			found = true
			if fi.Size != int64(len(testContent)) {
				t.Errorf("expected file size %d in listing, got %d", len(testContent), fi.Size)
			}
			if fi.IsDir {
				t.Errorf("uploaded file should not be marked IsDir")
			}
			break
		}
	}
	if !found {
		t.Fatalf("uploaded file %s not found in /tmp listing", testFilename)
	}

	// 5. Read the file
	readBytes, readFi, err := svc.ReadContainerFile(ctx, containerID, testFilePath, 1024)
	if err != nil {
		t.Fatalf("failed to read uploaded file: %v", err)
	}
	if string(readBytes) != testContent {
		t.Fatalf("expected read content %q, got %q", testContent, string(readBytes))
	}
	if readFi.Name != testFilename {
		t.Errorf("expected FileInfo name %q, got %q", testFilename, readFi.Name)
	}

	// 6. Download the file
	rc, downFi, err := svc.DownloadContainerFile(ctx, containerID, testFilePath)
	if err != nil {
		t.Fatalf("failed to download uploaded file: %v", err)
	}
	defer rc.Close()

	var downBuf bytes.Buffer
	if _, err := io.Copy(&downBuf, rc); err != nil {
		t.Fatalf("failed reading download stream: %v", err)
	}
	if downBuf.String() != testContent {
		t.Fatalf("download stream mismatch: expected %q, got %q", testContent, downBuf.String())
	}
	if downFi.Size != int64(len(testContent)) {
		t.Errorf("expected download FileInfo size %d, got %d", len(testContent), downFi.Size)
	}

	// 7. Delete the file
	err = svc.DeleteContainerFile(ctx, containerID, testFilePath)
	if err != nil {
		t.Fatalf("failed to delete test file: %v", err)
	}

	// Verify stat returns not found after delete
	_, err = svc.StatContainerPath(ctx, containerID, testFilePath)
	if err == nil {
		t.Errorf("expected error stating deleted file %s", testFilePath)
	}
}

func TestFiles_StoppedContainerTarFallback(t *testing.T) {
	svc, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create docker service: %v", err)
	}
	defer svc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	if err := svc.Ping(ctx); err != nil {
		t.Skipf("Docker daemon not accessible: %v", err)
	}

	// Create a stopped container
	cid, err := svc.CreateContainer(ctx, docker.CreateContainerRequest{
		Image: "alpine",
	})
	if err != nil {
		t.Skipf("Unable to create alpine container: %v", err)
	}
	defer func() {
		_ = svc.RemoveContainer(ctx, cid, true, false)
	}()

	// 1. Upload file into stopped container
	filename := "stopped_test.txt"
	content := "Written while stopped"
	err = svc.UploadContainerFile(ctx, cid, "/tmp", filename, strings.NewReader(content), int64(len(content)), 0644)
	if err != nil {
		t.Fatalf("failed uploading to stopped container: %v", err)
	}

	// 2. List files in /tmp on stopped container (exercises tar stream fallback)
	files, err := svc.ListContainerFiles(ctx, cid, "/tmp")
	if err != nil {
		t.Fatalf("failed listing files on stopped container: %v", err)
	}

	var found bool
	for _, fi := range files {
		if fi.Name == filename {
			found = true
			if fi.Size != int64(len(content)) {
				t.Errorf("expected size %d, got %d", len(content), fi.Size)
			}
			break
		}
	}
	if !found {
		t.Fatalf("file %s not found in stopped container listing", filename)
	}

	// 3. Read file on stopped container
	readData, _, err := svc.ReadContainerFile(ctx, cid, "/tmp/"+filename, 1024)
	if err != nil {
		t.Fatalf("failed reading file from stopped container: %v", err)
	}
	if string(readData) != content {
		t.Errorf("expected content %q, got %q", content, string(readData))
	}

	// 4. Attempt DeleteContainerFile on stopped container -> must return error because container is not running
	err = svc.DeleteContainerFile(ctx, cid, "/tmp/"+filename)
	if err == nil {
		t.Fatal("expected error deleting file on stopped container, got nil")
	}
	if !strings.Contains(err.Error(), "not running") {
		t.Errorf("expected 'not running' error, got %v", err)
	}
}
