import Modal from './Modal';
import styles from './HowItWorksModal.module.css';

interface HowItWorksModalProps {
  isOpen: boolean;
  onClose: () => void;
}

function HowItWorksDiagram() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 680 400" className={styles.diagram}>
      <defs>
        <filter id="shadow" x="-20%" y="-20%" width="140%" height="140%">
          <feDropShadow dx="0" dy="2" stdDeviation="4" floodColor="#000" floodOpacity="0.1"/>
        </filter>
        <marker id="arrowhead" markerWidth="8" markerHeight="6" refX="7" refY="3" orient="auto">
          <polygon points="0 0, 8 3, 0 6" fill="#94a3b8"/>
        </marker>
        <marker id="arrowheadGreen" markerWidth="8" markerHeight="6" refX="7" refY="3" orient="auto">
          <polygon points="0 0, 8 3, 0 6" fill="#6ee7b7"/>
        </marker>
      </defs>

      {/* Background */}
      <rect width="680" height="400" fill="#f8fafc"/>

      {/* LOCAL SECTION */}
      <g transform="translate(30, 20)">
        <rect x="0" y="0" width="300" height="320" rx="10" fill="#ffffff" filter="url(#shadow)" stroke="#e2e8f0" strokeWidth="1"/>
        <rect x="12" y="12" width="55" height="22" rx="4" fill="#f1f5f9"/>
        <text x="40" y="27" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="10" fontWeight="600" fill="#64748b">LOCAL</text>

        {/* Claude Code Session */}
        <rect x="15" y="48" width="270" height="38" rx="6" fill="#f5c9b8"/>
        <text x="150" y="72" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="14" fontWeight="600" fill="#1a1a1a">Claude Code Session</text>

        {/* Confab Sidecar */}
        <rect x="15" y="150" width="270" height="38" rx="6" fill="#ffffff" stroke="#e2e8f0" strokeWidth="1"/>
        <text x="150" y="174" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="12" fontWeight="600" fill="#1a1a1a">confab session sidecar</text>

        {/* Session Transcripts */}
        <rect x="15" y="248" width="270" height="38" rx="6" fill="#f5c9b8"/>
        <text x="150" y="272" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="12" fontWeight="500" fill="#1a1a1a">Session Transcripts (JSONL)</text>

        {/* Arrows */}
        <path d="M 80 86 L 80 148" stroke="#94a3b8" strokeWidth="1.5" fill="none" markerEnd="url(#arrowhead)"/>
        <text x="18" y="122" fontFamily="system-ui, sans-serif" fontSize="10" fontWeight="500" fill="#94a3b8">start hook</text>
        <path d="M 220 86 L 220 148" stroke="#94a3b8" strokeWidth="1.5" fill="none" markerEnd="url(#arrowhead)"/>
        <text x="235" y="122" fontFamily="system-ui, sans-serif" fontSize="10" fontWeight="500" fill="#94a3b8">stop hook</text>
        <path d="M 150 188 L 150 246" stroke="#94a3b8" strokeWidth="1.5" fill="none" markerEnd="url(#arrowhead)"/>
        <text x="170" y="222" fontFamily="system-ui, sans-serif" fontSize="10" fontWeight="500" fill="#94a3b8">watches</text>
      </g>

      {/* WEB APP SECTION */}
      <g transform="translate(390, 20)">
        <rect x="0" y="0" width="260" height="156" rx="10" fill="#ffffff" filter="url(#shadow)" stroke="#e2e8f0" strokeWidth="1"/>
        <rect x="0" y="0" width="260" height="30" rx="10" fill="#f1f5f9"/>
        <rect x="0" y="16" width="260" height="14" fill="#f1f5f9"/>
        <circle cx="18" cy="15" r="4" fill="#fca5a5"/>
        <circle cx="32" cy="15" r="4" fill="#fcd34d"/>
        <circle cx="46" cy="15" r="4" fill="#86efac"/>
        <rect x="70" y="8" width="120" height="14" rx="3" fill="#ffffff" stroke="#e2e8f0" strokeWidth="1"/>
        <text x="130" y="18" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="9" fill="#64748b">{window.location.host}</text>

        <g transform="translate(20, 42)">
          <circle cx="5" cy="8" r="3" fill="#3b82f6"/>
          <text x="16" y="11" fontFamily="system-ui, sans-serif" fontSize="12" fill="#1e293b">Session Listing</text>
          <g transform="translate(0, 28)">
            <circle cx="5" cy="8" r="3" fill="#8b5cf6"/>
            <text x="16" y="11" fontFamily="system-ui, sans-serif" fontSize="12" fill="#1e293b">Transcript View</text>
          </g>
          <g transform="translate(0, 56)">
            <circle cx="5" cy="8" r="3" fill="#10b981"/>
            <text x="16" y="11" fontFamily="system-ui, sans-serif" fontSize="12" fill="#1e293b">Metrics</text>
          </g>
          <g transform="translate(0, 84)">
            <circle cx="5" cy="8" r="3" fill="#f59e0b"/>
            <text x="16" y="11" fontFamily="system-ui, sans-serif" fontSize="12" fill="#1e293b">Sharing</text>
          </g>
        </g>
      </g>

      {/* BACKEND SECTION */}
      <g transform="translate(390, 210)">
        <rect x="0" y="0" width="260" height="50" rx="6" fill="#ffffff" filter="url(#shadow)" stroke="#e2e8f0" strokeWidth="1"/>
        <text x="130" y="22" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="14" fontWeight="600" fill="#1a1a1a">{window.location.host}</text>
        <text x="130" y="38" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="11" fill="#64748b">Backend API</text>
      </g>

      {/* DATA LAYER */}
      <g transform="translate(390, 290)">
        <g transform="translate(0, 0)">
          <path d="M 10 15 L 10 55 A 50 14 0 0 0 110 55 L 110 15" fill="#dbeafe" stroke="none"/>
          <ellipse cx="60" cy="55" rx="50" ry="14" fill="#c5d9f5"/>
          <ellipse cx="60" cy="15" rx="50" ry="14" fill="#e8f2fc"/>
          <text x="60" y="38" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="10" fontStyle="italic" fill="#1a1a1a">DB</text>
          <text x="60" y="52" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="9" fill="#64748b">sessions index</text>
        </g>
        <g transform="translate(140, 0)">
          <path d="M 10 15 L 10 55 A 50 14 0 0 0 110 55 L 110 15" fill="#fee4cb" stroke="none"/>
          <ellipse cx="60" cy="55" rx="50" ry="14" fill="#f5d4b8"/>
          <ellipse cx="60" cy="15" rx="50" ry="14" fill="#fef0e4"/>
          <text x="60" y="38" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="10" fontStyle="italic" fill="#1a1a1a">Object Store</text>
          <text x="60" y="52" textAnchor="middle" fontFamily="system-ui, sans-serif" fontSize="9" fill="#64748b">transcripts</text>
        </g>
      </g>

      {/* CONNECTING ARROWS */}
      <path d="M 315 189 L 390 235" stroke="#6ee7b7" strokeWidth="2" fill="none" markerEnd="url(#arrowheadGreen)" strokeDasharray="6,4"/>
      <text x="340" y="198" fontFamily="system-ui, sans-serif" fontSize="11" fontWeight="500" fill="#10b981" transform="rotate(32, 340, 198)">sync</text>
      <path d="M 520 176 L 520 208" stroke="#94a3b8" strokeWidth="1.5" fill="none" markerEnd="url(#arrowhead)"/>
      <path d="M 450 260 L 450 288" stroke="#94a3b8" strokeWidth="1.5" fill="none" markerEnd="url(#arrowhead)"/>
      <path d="M 590 260 L 590 288" stroke="#94a3b8" strokeWidth="1.5" fill="none" markerEnd="url(#arrowhead)"/>
    </svg>
  );
}

function HowItWorksModal({ isOpen, onClose }: HowItWorksModalProps) {
  return (
    <Modal isOpen={isOpen} onClose={onClose} className={styles.modal} ariaLabel="How it works">
      <h2 className={styles.title}>How it works</h2>
      <HowItWorksDiagram />
    </Modal>
  );
}

export default HowItWorksModal;
