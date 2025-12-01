import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { CheckCircle2 } from 'lucide-react';

interface SuccessBannerProps {
  title?: string;
  message: string;
}

export const SuccessBanner = ({ title = 'Success', message }: SuccessBannerProps) => {
  return (
    <Alert className="animate-fade-in border-primary/50 bg-primary/5">
      <CheckCircle2 className="h-4 w-4 text-primary" />
      <AlertTitle className="text-primary">{title}</AlertTitle>
      <AlertDescription>{message}</AlertDescription>
    </Alert>
  );
};
