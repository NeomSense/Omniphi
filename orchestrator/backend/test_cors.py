"""Test CORS configuration."""
from app.core.config import settings

print("=" * 60)
print("CORS Configuration Test")
print("=" * 60)
print(f"\nBACKEND_CORS_ORIGINS type: {type(settings.BACKEND_CORS_ORIGINS)}")
print(f"\nBACKEND_CORS_ORIGINS value:")
for i, origin in enumerate(settings.BACKEND_CORS_ORIGINS):
    print(f"  [{i}] {repr(origin)} (type: {type(origin)})")

print(f"\nChecking if 'http://localhost:8080' is in list:")
print(f"  'http://localhost:8080' in settings.BACKEND_CORS_ORIGINS: {('http://localhost:8080' in settings.BACKEND_CORS_ORIGINS)}")

print("\nDirect string comparison tests:")
target = "http://localhost:8080"
for origin in settings.BACKEND_CORS_ORIGINS:
    match = (origin == target)
    print(f"  '{origin}' == '{target}': {match}")
