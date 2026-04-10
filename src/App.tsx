import { useState } from 'react';
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

  const updateCreationDetails = (field: keyof CreationDetails, value: string) => {
    setCreationDetails((details) => ({ ...details, [field]: value }));
  };

  const handleCreationSubmit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (!selectedArea) {
      return;
    }

    const creationPayload = {
      selection: selectedArea,
      details: creationDetails,
    };

    console.log('Creation payload ready for backend:', creationPayload);
  };

  return (
    <div className="app">
      <Map onSelectionChange={setSelectedArea} />
      <img className="tv-overlay" src="/gfx/tv.png" alt="" aria-hidden="true" />
      <h1 className="app-title">7 layers of hell</h1>
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
          <button className="prompt-submit" type="submit">
            Add
          </button>
        </form>
      )}
      <output className="selection-debug" aria-live="polite">
        {selectedArea
          ? `${selectedArea.geoBounds.west.toFixed(5)}, ${selectedArea.geoBounds.south.toFixed(
              5,
            )} - ${selectedArea.geoBounds.east.toFixed(5)}, ${selectedArea.geoBounds.north.toFixed(5)}`
          : 'Drag to select an area'}
      </output>
    </div>
  );
}

export default App;
