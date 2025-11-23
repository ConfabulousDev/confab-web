import styles from './FormField.module.css';

interface FormFieldProps {
  label: string;
  error?: string;
  required?: boolean;
  children: React.ReactNode;
}

/**
 * Reusable form field wrapper with label and error display
 */
function FormField({ label, error, required, children }: FormFieldProps) {
  return (
    <div className={styles.formField}>
      <label className={styles.label}>
        {label}
        {required && <span className={styles.required}>*</span>}
      </label>
      {children}
      {error && <div className={styles.error}>{error}</div>}
    </div>
  );
}

export default FormField;
