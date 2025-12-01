import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { AlertCircle } from 'lucide-react';

interface ErrorBannerProps {
  title?: string;
  message: string;
}

export const ErrorBanner = ({ title = 'Error', message }: ErrorBannerProps) => {
  return (
    <Alert variant="destructive" className="animate-fade-in">
      <AlertCircle className="h-4 w-4" />
      <AlertTitle>{title}</AlertTitle>
      <AlertDescription>{message}</AlertDescription>
    </Alert>
  );
};
