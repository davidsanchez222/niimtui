package catalog

import "testing"

func TestKnownModelsIncludesSetupModels(t *testing.T) {
	models := KnownModels()
	want := []string{"M2", "M3", "N1", "B21 Pro", "B1", "B4", "B2 Pro", "B1 Pro", "B2", "B21", "B3S", "K3", "D11", "D110", "D101"}
	if len(models) != len(want) {
		t.Fatalf("model count = %d, want %d", len(models), len(want))
	}
	for i := range want {
		if models[i] != want[i] {
			t.Fatalf("models[%d] = %q, want %q", i, models[i], want[i])
		}
	}
}

func TestLabelSizesForSharedBSeriesModels(t *testing.T) {
	base, err := LabelSizesForModel("B1")
	if err != nil {
		t.Fatalf("LabelSizesForModel(B1) error = %v", err)
	}
	for _, model := range []string{"B2", "B21", "B3S", "K3"} {
		sizes, err := LabelSizesForModel(model)
		if err != nil {
			t.Fatalf("LabelSizesForModel(%s) error = %v", model, err)
		}
		if len(sizes) != len(base) {
			t.Fatalf("%s size count = %d, want B1 count %d", model, len(sizes), len(base))
		}
	}
}

func TestLabelSizesForB4(t *testing.T) {
	sizes, err := LabelSizesForModel("B4")
	if err != nil {
		t.Fatalf("LabelSizesForModel(B4) error = %v", err)
	}
	if len(sizes) != 1 {
		t.Fatalf("B4 size count = %d, want 1", len(sizes))
	}
	if sizes[0].WidthMM != 100 || sizes[0].HeightMM != 150 || sizes[0].Shape != "rect" {
		t.Fatalf("B4 size = %#v, want 100x150 rect", sizes[0])
	}
}

func TestLabelPresetsForModelUsesStableNames(t *testing.T) {
	presets, err := LabelPresetsForModel("B1")
	if err != nil {
		t.Fatalf("LabelPresetsForModel(B1) error = %v", err)
	}
	want := map[string]struct{}{
		"b1-50x30":        {},
		"b1-50x50-round":  {},
		"b1-50x50-square": {},
		"b1-50x15-2lane":  {},
	}
	for _, preset := range presets {
		delete(want, preset.Name)
	}
	for name := range want {
		t.Fatalf("missing preset %q", name)
	}
}
