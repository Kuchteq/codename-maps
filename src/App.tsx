import { useEffect, useState } from 'react';
import { apiClient } from './api';
import type { Edit, EditRequest } from './api';
import Map from './components/Map';
import type { MapSelection } from './components/Map';
import './App.css';

interface CreationDetails {
  name: string;
  author: string;
  prompt: string;
}

function App() {
  const [selectedArea, setSelectedArea] = useState<MapSelection | null>(null);
  const [creationDetails, setCreationDetails] = useState<CreationDetails>({
    name: '',
    author: '',
    prompt: '',
  });
  const [submitStatus, setSubmitStatus] = useState<'idle' | 'submitting' | 'sent' | 'error'>(
    'idle',
  );
  const [tileRevision, setTileRevision] = useState(() => Date.now());
  const [edits, setEdits] = useState<Edit[]>([]);

  const updateCreationDetails = (field: keyof CreationDetails, value: string) => {
    setCreationDetails((details) => ({ ...details, [field]: value }));
    setSubmitStatus('idle');
  };

  const handleSelectionChange = (selection: MapSelection | null) => {
    setSelectedArea(selection);
    setSubmitStatus('idle');
  };

  const refreshEdits = async () => {
    const { data, error } = await apiClient.GET('/v1/edits');

    if (error) {
      console.error('Failed to load edits:', error);
      return;
    }

    setEdits((previousEdits) => {
      const previousSignature = previousEdits.map((edit) => `${edit.id}:${edit.createdAt}`).join('|');
      const nextSignature = data.map((edit) => `${edit.id}:${edit.createdAt}`).join('|');

      if (previousSignature !== nextSignature) {
        setTileRevision(Date.now());
      }

      return data;
    });
  };

  useEffect(() => {
    void refreshEdits();

    const interval = window.setInterval(() => {
      void refreshEdits();
    }, 3000);

    return () => window.clearInterval(interval);
  }, []);

  const handleCreationSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (!selectedArea) {
      return;
    }

    const editRequest: EditRequest = {
      name: creationDetails.name,
      author: creationDetails.author,
      prompt: creationDetails.prompt,
      start: {
        type: 'Point',
        coordinates: [selectedArea.geoBounds.west, selectedArea.geoBounds.south],
      },
      end: {
        type: 'Point',
        coordinates: [selectedArea.geoBounds.east, selectedArea.geoBounds.north],
      },
    };

    setSubmitStatus('submitting');

    const { error } = await apiClient.POST('/v1/edit', {
      body: editRequest,
    });

    if (error) {
      setSubmitStatus('error');
      console.error('Failed to submit edit:', error);
      return;
    }

    setSubmitStatus('sent');
    await refreshEdits();
  };

  return (
    <div className="app">
      <Map edits={edits} tileRevision={tileRevision} onSelectionChange={handleSelectionChange} />
      <img className="tv-overlay" src="/gfx/tv.png" alt="" aria-hidden="true" />
      <h1 className="app-title">7 layers of hel</h1>
      {selectedArea && (
        <form className="prompt-box" onSubmit={handleCreationSubmit}>
          <input
            className="prompt-input prompt-input-name"
            name="creation-name"
            type="text"
            placeholder="Name of creation"
            value={creationDetails.name}
            onChange={(event) => updateCreationDetails('name', event.target.value)}
          />
          <input
            className="prompt-input prompt-input-author"
            name="author"
            type="text"
            placeholder="Author"
            value={creationDetails.author}
            onChange={(event) => updateCreationDetails('author', event.target.value)}
          />
          <textarea
            className="prompt-input prompt-input-text"
            name="prompt"
            placeholder="Create whatever you want here..."
            value={creationDetails.prompt}
            onChange={(event) => updateCreationDetails('prompt', event.target.value)}
            rows={1}
          />
          <button className="prompt-submit" type="submit" disabled={submitStatus === 'submitting'}>
            {submitStatus === 'submitting' ? 'Adding' : 'Add'}
          </button>
          <p className="submit-status" role="status">
            {submitStatus === 'sent' && 'Sent'}
            {submitStatus === 'error' && 'Could not send'}
          </p>
        </form>
      )}
      <output className="selection-debug" aria-live="polite">
        {selectedArea
          ? `${selectedArea.geoBounds.west.toFixed(5)}, ${selectedArea.geoBounds.south.toFixed(
              5,
            )} - ${selectedArea.geoBounds.east.toFixed(5)}, ${selectedArea.geoBounds.north.toFixed(5)}`
          : 'Drag to select an area'}
      </output>
      <section className="creations-panel" aria-label="Recent creations">
        <h2>Creations</h2>
        <p>{edits.length ? `${edits.length} live` : 'No creations yet'}</p>
        {edits.slice(0, 4).map((edit) => (
          <article key={edit.id}>
            <strong>{edit.name}</strong>
            <span>{edit.author}</span>
          </article>
        ))}
      </section>
    </div>
  );
}

export default App;
