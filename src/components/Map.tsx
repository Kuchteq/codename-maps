import { useRef, useState } from 'react';
import type { PointerEvent } from 'react';
import { Layer, Map as MapLibre, Source } from 'react-map-gl/maplibre';
import type { MapRef } from 'react-map-gl/maplibre';
import type { FillLayerSpecification, LineLayerSpecification, StyleSpecification } from 'maplibre-gl';
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

const SELECTED_AREA_FILL: FillLayerSpecification = {
  id: 'selected-area-fill',
  type: 'fill',
  source: 'selected-area',
  paint: {
    'fill-color': '#ffffff',
    'fill-opacity': 0.2,
  },
};

const SELECTED_AREA_OUTLINE: LineLayerSpecification = {
  id: 'selected-area-outline',
  type: 'line',
  source: 'selected-area',
  paint: {
    'line-color': '#ffffff',
    'line-width': 2,
    'line-opacity': 0.95,
    'line-blur': 0.5,
  },
};

interface ScreenPoint {
  x: number;
  y: number;
}

interface GeoPoint {
  longitude: number;
  latitude: number;
}

export interface MapSelection {
  screenCorners: {
    topLeft: ScreenPoint;
    topRight: ScreenPoint;
    bottomRight: ScreenPoint;
    bottomLeft: ScreenPoint;
  };
  geoCorners: {
    topLeft: GeoPoint;
    topRight: GeoPoint;
    bottomRight: GeoPoint;
    bottomLeft: GeoPoint;
  };
  geoBounds: {
    west: number;
    south: number;
    east: number;
    north: number;
  };
}

interface MapProps {
  onSelectionChange?: (selection: MapSelection | null) => void;
}

interface DragState {
  start: ScreenPoint;
  current: ScreenPoint;
}

const getSelectionRect = (start: ScreenPoint, current: ScreenPoint) => {
  const left = Math.min(start.x, current.x);
  const top = Math.min(start.y, current.y);
  const width = Math.abs(current.x - start.x);
  const height = Math.abs(current.y - start.y);

  return { left, top, width, height };
};

const getPointFromEvent = (event: PointerEvent<HTMLDivElement>): ScreenPoint => {
  const bounds = event.currentTarget.getBoundingClientRect();

  return {
    x: event.clientX - bounds.left,
    y: event.clientY - bounds.top,
  };
};

export default function Map({ onSelectionChange }: MapProps) {
  const mapRef = useRef<MapRef>(null);
  const [drag, setDrag] = useState<DragState | null>(null);
  const [selection, setSelection] = useState<MapSelection | null>(null);

  const finishSelection = (start: ScreenPoint, current: ScreenPoint) => {
    const rect = getSelectionRect(start, current);

    if (rect.width < 4 || rect.height < 4) {
      setSelection(null);
      onSelectionChange?.(null);
      return;
    }

    const map = mapRef.current?.getMap();
    if (!map) {
      return;
    }

    const screenCorners = {
      topLeft: { x: rect.left, y: rect.top },
      topRight: { x: rect.left + rect.width, y: rect.top },
      bottomRight: { x: rect.left + rect.width, y: rect.top + rect.height },
      bottomLeft: { x: rect.left, y: rect.top + rect.height },
    };

    const toGeoPoint = (point: ScreenPoint): GeoPoint => {
      const lngLat = map.unproject([point.x, point.y]);

      return {
        longitude: lngLat.lng,
        latitude: lngLat.lat,
      };
    };

    const geoCorners = {
      topLeft: toGeoPoint(screenCorners.topLeft),
      topRight: toGeoPoint(screenCorners.topRight),
      bottomRight: toGeoPoint(screenCorners.bottomRight),
      bottomLeft: toGeoPoint(screenCorners.bottomLeft),
    };

    const longitudes = Object.values(geoCorners).map((point) => point.longitude);
    const latitudes = Object.values(geoCorners).map((point) => point.latitude);

    const nextSelection = {
      screenCorners,
      geoCorners,
      geoBounds: {
        west: Math.min(...longitudes),
        south: Math.min(...latitudes),
        east: Math.max(...longitudes),
        north: Math.max(...latitudes),
      },
    };

    setSelection(nextSelection);
    onSelectionChange?.(nextSelection);
  };

  const selectedAreaGeoJson = selection
    ? {
        type: 'Feature' as const,
        properties: {},
        geometry: {
          type: 'Polygon' as const,
          coordinates: [
            [
              [selection.geoCorners.topLeft.longitude, selection.geoCorners.topLeft.latitude],
              [selection.geoCorners.topRight.longitude, selection.geoCorners.topRight.latitude],
              [
                selection.geoCorners.bottomRight.longitude,
                selection.geoCorners.bottomRight.latitude,
              ],
              [selection.geoCorners.bottomLeft.longitude, selection.geoCorners.bottomLeft.latitude],
              [selection.geoCorners.topLeft.longitude, selection.geoCorners.topLeft.latitude],
            ],
          ],
        },
      }
    : null;

  return (
    <div
      className={`map-shell${drag ? ' is-selecting' : ''}`}
      onPointerDownCapture={(event) => {
        if (event.button !== 0 || !event.shiftKey) {
          return;
        }

        event.preventDefault();
        event.stopPropagation();
        const point = getPointFromEvent(event);
        event.currentTarget.setPointerCapture(event.pointerId);
        setDrag({ start: point, current: point });
      }}
      onPointerMoveCapture={(event) => {
        if (!drag) {
          return;
        }

        event.preventDefault();
        event.stopPropagation();
        setDrag({ ...drag, current: getPointFromEvent(event) });
      }}
      onPointerUpCapture={(event) => {
        if (!drag) {
          return;
        }

        event.preventDefault();
        event.stopPropagation();
        const current = getPointFromEvent(event);
        finishSelection(drag.start, current);
        setDrag(null);
      }}
      onPointerCancelCapture={() => {
        setDrag(null);
      }}
    >
      <MapLibre
        ref={mapRef}
        initialViewState={INITIAL_VIEW}
        style={{ width: '100%', height: '100%' }}
        mapStyle={MAP_STYLE}
        maxBounds={HELSINKI_BOUNDS}
        minZoom={10}
        maxZoom={20}
        attributionControl={false}
      >
        {selectedAreaGeoJson && (
          <Source id="selected-area" type="geojson" data={selectedAreaGeoJson}>
            <Layer {...SELECTED_AREA_FILL} />
            <Layer {...SELECTED_AREA_OUTLINE} />
          </Source>
        )}

      <Source
        id="edits"
        type="raster"
        url="http://localhost:8080/tiles.json"
        tileSize={512}
      >
        <Layer
          id="edits-layer"
          type="raster"
        />
      </Source>
      </MapLibre>
      <div className="selection-layer">
        {drag && (
          <div
            className="selection-rectangle"
            style={getSelectionRect(drag.start, drag.current)}
          />
        )}
      </div>
    </div>
  );
}
