GEOJSON = natural-earth-vector/geojson
DATAGEN = go run -tags datagen ./cmd/datagen
GEODATA = \
	data/Cities10.gz \
	data/Countries10.gz \
	data/Countries110.gz \
	data/Provinces10.gz

all: geodata

geodata: $(GEODATA)

clean:
	rm -f $(GEODATA)

data/Cities10.gz data/Cities10.txt: $(GEOJSON)/ne_10m_urban_areas_landscan.geojson
	$(DATAGEN) -o $@ $^

data/Countries10.gz data/Countries10.txt: $(GEOJSON)/ne_10m_admin_0_countries.geojson
	$(DATAGEN) -o $@ $^

data/Countries110.gz data/Countries110.txt: $(GEOJSON)/ne_110m_admin_0_countries.geojson
	$(DATAGEN) -o $@ $^

data/Provinces10.gz data/Provinces10.txt: $(GEOJSON)/ne_10m_admin_0_countries.geojson $(GEOJSON)/ne_10m_admin_1_states_provinces.geojson
	$(DATAGEN) -o $@ -merge $^

.PHONY: all clean geodata
