import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { ModeSelection } from '@/components/ModeSelection';
import { ValidatorWizard } from '@/components/ValidatorWizard';
import { WalletConnect } from '@/components/WalletConnect';
import { useValidatorStore } from '@/store/validatorStore';
import { Shield } from 'lucide-react';

const Wizard = () => {
  const navigate = useNavigate();
  const { isWalletConnected, validatorMode } = useValidatorStore();

  useEffect(() => {
    if (!isWalletConnected) {
      navigate('/');
    }
  }, [isWalletConnected, navigate]);

  return (
    <div className="min-h-screen bg-gradient-to-b from-background via-background to-background/80">
      {/* Header */}
      <header className="border-b border-border/50 backdrop-blur-sm sticky top-0 z-50 bg-background/80">
        <div className="container mx-auto px-4 py-4 flex items-center justify-between">
          <button 
            onClick={() => navigate('/')}
            className="flex items-center gap-2 group transition-all hover:opacity-80 cursor-pointer"
          >
            <div className="h-10 w-10 rounded-lg bg-gradient-to-br from-primary to-accent flex items-center justify-center group-hover:scale-105 transition-transform">
              <Shield className="h-6 w-6 text-primary-foreground" />
            </div>
            <span className="text-xl font-bold gradient-text">Omniphi Validators</span>
          </button>
          <WalletConnect />
        </div>
      </header>

      {/* Main Content */}
      <main className="container mx-auto px-4 py-12">
        {!validatorMode ? (
          <ModeSelection onModeSelected={() => {}} />
        ) : (
          <ValidatorWizard onComplete={() => navigate('/dashboard')} />
        )}
      </main>

      {/* Footer */}
      <footer className="border-t border-border/50 mt-24">
        <div className="container mx-auto px-4 py-6">
          <div className="flex items-center justify-between text-sm text-muted-foreground">
            <p>&copy; 2024 Omniphi. All rights reserved.</p>
            <div className="flex gap-6">
              <a href="#" className="hover:text-primary transition-colors">Documentation</a>
              <a href="#" className="hover:text-primary transition-colors">Support</a>
              <a href="#" className="hover:text-primary transition-colors">Terms</a>
            </div>
          </div>
        </div>
      </footer>
    </div>
  );
};

export default Wizard;
