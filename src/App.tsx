import Map from './components/Map';
import Toolbar from './components/Toolbar';
import './App.css';

function App() {
  return (
    <div className="app">
      <Map />
      <img className="tv-overlay" src="/gfx/tv.png" alt="" aria-hidden="true" />
      <h1 className="app-title">7 layers of hell</h1>
      <form className="prompt-box">
        <textarea
          id="prompt-input"
          className="prompt-input"
          placeholder="Create whatever you want here..."
          rows={1}
        />
      </form>
    </div>
  );
}

export default App;
