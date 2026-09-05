package investigation

import "testing"

func TestPickWithinBudgetDoesNotSwallowAll(t *testing.T) {
	contents := map[string]string{}
	names := make([]string, 0, 12)
	for i := 0; i < 12; i++ {
		name := "blob_" + string(rune('a'+i)) + ".txt"
		names = append(names, name)
		body := make([]byte, 4000)
		for j := range body {
			body[j] = 'x'
		}
		contents[name] = string(body)
	}
	contents["training.log"] = "CUDA out of memory\n"
	names = append(names, "training.log")

	ranked := rankNames(names, "GPU OOM during training", "memory_gpu")
	if ranked[0] != "training.log" {
		t.Fatalf("expected training.log first, got %v", ranked)
	}
	picked := pickWithinBudget(ranked, contents, 9000)
	if len(picked) >= len(contents) {
		t.Fatalf("expected budget to drop files, picked %d of %d", len(picked), len(contents))
	}
	total := 0
	for _, body := range picked {
		total += len(body)
	}
	if total > 9000 {
		t.Fatalf("over budget: %d", total)
	}
	if _, ok := picked["training.log"]; !ok {
		t.Fatal("highest-ranked file must be kept")
	}
}

func TestFileScorePrefersLeakageSources(t *testing.T) {
	names := []string{"notes.txt", "pipeline.py", "metrics.json", "readme.txt"}
	ranked := rankNames(names, "Why did accuracy collapse?", "data_leakage")
	if ranked[0] != "pipeline.py" {
		t.Fatalf("pipeline.py should rank first, got %v", ranked)
	}
}
