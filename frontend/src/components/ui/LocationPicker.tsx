import { useRef, useState } from 'react'
import { MapContainer, TileLayer, Marker, useMapEvents } from 'react-leaflet'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'

// Fix Leaflet's default marker icon broken by Vite's asset pipeline
const defaultIcon = new L.Icon({
  iconUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon.png',
  iconRetinaUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon-2x.png',
  shadowUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-shadow.png',
  iconSize: [25, 41],
  iconAnchor: [12, 41],
  popupAnchor: [1, -34],
  shadowSize: [41, 41],
})
L.Marker.prototype.options.icon = defaultIcon

// Default centre when geolocation is unavailable (Vancouver, Canada)
const DEFAULT_CENTER: [number, number] = [49.2827, -123.1207]
const DEFAULT_ZOOM = 13

interface Props {
  lat: number | null
  lng: number | null
  onChange: (lat: number, lng: number) => void
}

/** Captures click events on the map to drop a pin. */
function ClickHandler({ onChange }: { onChange: (lat: number, lng: number) => void }) {
  useMapEvents({
    click(e) {
      onChange(e.latlng.lat, e.latlng.lng)
    },
  })
  return null
}

/** Draggable marker — updates coords when dragged. */
function DraggableMarker({ lat, lng, onChange }: { lat: number; lng: number; onChange: (lat: number, lng: number) => void }) {
  const markerRef = useRef<L.Marker>(null)
  return (
    <Marker
      position={[lat, lng]}
      draggable
      ref={markerRef}
      eventHandlers={{
        dragend() {
          const marker = markerRef.current
          if (marker) {
            const pos = marker.getLatLng()
            onChange(pos.lat, pos.lng)
          }
        },
      }}
    />
  )
}

export function LocationPicker({ lat, lng, onChange }: Props) {
  const [geoStatus, setGeoStatus] = useState<'idle' | 'loading' | 'set' | 'denied' | 'timeout'>(() =>
    lat !== null && lng !== null ? 'set' : 'idle'
  )
  const [showMap, setShowMap] = useState(lat !== null && lng !== null)
  // mapKey forces MapContainer to remount at the detected location after auto-detect
  const [mapKey, setMapKey] = useState(0)

  const detectLocation = () => {
    if (!navigator.geolocation) { setGeoStatus('denied'); setShowMap(true); return }
    setGeoStatus('loading')
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        onChange(pos.coords.latitude, pos.coords.longitude)
        setGeoStatus('set')
        setShowMap(true)
        setMapKey((k) => k + 1) // remount map centred on detected location
      },
      (err) => {
        setGeoStatus(err.code === err.TIMEOUT ? 'timeout' : 'denied')
        setShowMap(true)
      },
      { enableHighAccuracy: false, timeout: 5000, maximumAge: 60000 },
    )
  }

  const mapCenter: [number, number] = lat !== null && lng !== null ? [lat, lng] : DEFAULT_CENTER

  return (
    <div className="flex flex-col gap-3">
      {/* Action buttons */}
      <div className="flex gap-2">
        <button
          type="button"
          onClick={detectLocation}
          disabled={geoStatus === 'loading'}
          className="flex items-center gap-2 px-4 py-2 rounded-xl border-2 border-border text-text-secondary text-sm font-medium hover:border-brand/50 hover:text-brand transition-colors disabled:opacity-50"
        >
          {geoStatus === 'loading' ? (
            <>
              <span className="inline-block w-3 h-3 border-2 border-current border-t-transparent rounded-full animate-spin" />
              Detecting…
            </>
          ) : (
            <>📍 Use my location</>
          )}
        </button>
        <button
          type="button"
          onClick={() => setShowMap((s) => !s)}
          className="flex items-center gap-2 px-4 py-2 rounded-xl border-2 border-border text-text-secondary text-sm font-medium hover:border-brand/50 hover:text-brand transition-colors"
        >
          🗺 {showMap ? 'Hide map' : 'Pin on map'}
        </button>
      </div>

      {/* Status line */}
      {geoStatus === 'set' && lat !== null && lng !== null && (
        <p className="text-xs text-brand font-medium">
          ✓ Location set ({lat.toFixed(5)}, {lng.toFixed(5)}) — drag the pin to adjust
        </p>
      )}
      {geoStatus === 'denied' && (
        <p className="text-xs text-red-500">
          Location access denied — use the map below to pin your shop manually.
        </p>
      )}
      {geoStatus === 'timeout' && (
        <p className="text-xs text-red-500">
          Location request timed out — use the map below to pin your shop manually.
        </p>
      )}
      {geoStatus === 'idle' && (
        <p className="text-xs text-text-secondary">
          Required for proximity search. Use auto-detect or pin on the map.
        </p>
      )}

      {/* Map */}
      {showMap && (
        <div className="rounded-xl overflow-hidden border border-border" style={{ height: 280 }}>
          <MapContainer
            key={mapKey}
            center={mapCenter}
            zoom={DEFAULT_ZOOM}
            style={{ height: '100%', width: '100%' }}
          >
            <TileLayer
              attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
              url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
            />
            <ClickHandler onChange={(la, ln) => { onChange(la, ln); setGeoStatus('set') }} />
            {lat !== null && lng !== null && (
              <DraggableMarker lat={lat} lng={lng} onChange={(la, ln) => { onChange(la, ln); setGeoStatus('set') }} />
            )}
          </MapContainer>
        </div>
      )}
    </div>
  )
}
