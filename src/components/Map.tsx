import { Map as MapLibre } from 'react-map-gl/maplibre';
import type { StyleSpecification } from 'maplibre-gl';
import 'maplibre-gl/dist/maplibre-gl.css';

const WMS_BASE = 'https://kartta.hel.fi/ws/geoserver/avoindata/wms';
const WMS_LAYER = 'avoindata:Ortoilmakuva_2025_5cm';

// WMS tile URL template in EPSG:3857 (Web Mercator — MapLibre's native CRS).
// MapLibre substitutes {bbox-epsg-3857} with the tile bounding box.
const wmsUrl =
  `${WMS_BASE}?SERVICE=WMS&VERSION=1.3.0&REQUEST=GetMap` +
  `&LAYERS=${encodeURIComponent(WMS_LAYER)}` +
  `&STYLES=` +
  `&FORMAT=image/png` +
  `&TRANSPARENT=TRUE` +
  `&CRS=EPSG:3857` +
  `&WIDTH=256&HEIGHT=256` +
  `&BBOX={bbox-epsg-3857}`;

const MAP_STYLE: StyleSpecification = {
  version: 8,
  sources: {
    'helsinki-ortho': {
      type: 'raster',
      tiles: [wmsUrl],
      tileSize: 256,
      // Extent of Ortoilmakuva_2025_5cm in WGS84 [minLng, minLat, maxLng, maxLat]
      bounds: [24.819, 60.124, 25.272, 60.305],
      attribution:
        '&copy; <a href="https://hri.fi/data/dataset/helsingin-ortoilmakuvat">Helsingin kaupunki</a>',
    },
  },
  layers: [
    {
      id: 'background',
      type: 'background',
      paint: { 'background-color': '#1a1a2e' },
    },
    {
      id: 'helsinki-ortho-layer',
      type: 'raster',
      source: 'helsinki-ortho',
      paint: { 'raster-opacity': 1 },
    },
  ],
};

// Helsinki bounding box [west, south, east, north] — matches ortho layer extent
const HELSINKI_BOUNDS: [number, number, number, number] = [24.819, 60.124, 25.272, 60.305];

// Helsinki city centre
const INITIAL_VIEW = {
  longitude: 25.004,
  latitude: 60.169,
  zoom: 12,
};

export default function Map() {
  return (
    <MapLibre
      initialViewState={INITIAL_VIEW}
      style={{ width: '100%', height: '100%' }}
      mapStyle={MAP_STYLE}
      maxBounds={HELSINKI_BOUNDS}
      minZoom={10}
      maxZoom={20}
      attributionControl={false}
    />
  );
}
