import styles from './Toolbar.module.css';

interface ToolbarButton {
  id: string;
  label: string;
  icon: string;
  onClick?: () => void;
}

const DEFAULT_TOOLS: ToolbarButton[] = [
  { id: 'search', label: 'Search', icon: '🔍' },
  { id: 'layers', label: 'Layers', icon: '🗂' },
  { id: 'location', label: 'My Location', icon: '📍' },
  { id: 'settings', label: 'Settings', icon: '⚙️' },
];

interface ToolbarProps {
  tools?: ToolbarButton[];
}

export default function Toolbar({ tools = DEFAULT_TOOLS }: ToolbarProps) {
  return (
    <div className={styles.toolbar}>
      {tools.map((tool) => (
        <button
          key={tool.id}
          className={styles.toolButton}
          title={tool.label}
          aria-label={tool.label}
          onClick={tool.onClick}
        >
          <span className={styles.icon}>{tool.icon}</span>
          <span className={styles.label}>{tool.label}</span>
        </button>
      ))}
    </div>
  );
}
