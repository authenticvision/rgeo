GEOJSON = natural-earth-vector/geojson
DATAGEN = go run -tags datagen ./cmd/datagen
GEODATA = \
	data/Cities10.zst \
	data/Countries10.zst \
	data/Countries110.zst \
	data/Provinces10.zst

all: geodata

geodata: $(GEODATA)

clean:
	rm -f $(GEODATA)

data/Cities10.zst data/Cities10.txt: $(GEOJSON)/ne_10m_urban_areas_landscan.geojson
	$(DATAGEN) -o $@ $^

data/Countries10.zst data/Countries10.txt: $(GEOJSON)/ne_10m_admin_0_countries.geojson
	$(DATAGEN) -o $@ $^

data/Countries110.zst data/Countries110.txt: $(GEOJSON)/ne_110m_admin_0_countries.geojson
	$(DATAGEN) -o $@ $^

data/Provinces10.zst data/Provinces10.txt: $(GEOJSON)/ne_10m_admin_0_countries.geojson $(GEOJSON)/ne_10m_admin_1_states_provinces.geojson
	$(DATAGEN) -o $@ -merge $^

.PHONY: all clean geodata
