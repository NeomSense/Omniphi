"""
Base Repository Class

Provides generic CRUD operations that can be inherited by model-specific repositories.
"""

from datetime import datetime
from typing import Any, Dict, Generic, List, Optional, Type, TypeVar, Union
from uuid import UUID

from sqlalchemy import and_, asc, desc, func, or_
from sqlalchemy.orm import Session

from app.db.database import Base

# Type variable for the model
ModelType = TypeVar("ModelType", bound=Base)


class BaseRepository(Generic[ModelType]):
    """
    Generic base repository providing CRUD operations.

    Usage:
        class UserRepository(BaseRepository[User]):
            def __init__(self, db: Session):
                super().__init__(User, db)

            def get_by_email(self, email: str) -> Optional[User]:
                return self.db.query(self.model).filter(self.model.email == email).first()
    """

    def __init__(self, model: Type[ModelType], db: Session):
        """
        Initialize repository with model class and database session.

        Args:
            model: SQLAlchemy model class
            db: Database session
        """
        self.model = model
        self.db = db

    def create(self, data: Union[Dict[str, Any], Any], **kwargs) -> ModelType:
        """
        Create a new record.

        Args:
            data: Dictionary or Pydantic model with field values
            **kwargs: Additional fields to set

        Returns:
            Created model instance
        """
        if hasattr(data, "model_dump"):
            # Pydantic model
            obj_data = data.model_dump(exclude_unset=True)
        elif hasattr(data, "dict"):
            # Pydantic v1 model
            obj_data = data.dict(exclude_unset=True)
        else:
            obj_data = dict(data)

        obj_data.update(kwargs)
        db_obj = self.model(**obj_data)
        self.db.add(db_obj)
        self.db.commit()
        self.db.refresh(db_obj)
        return db_obj

    def get(self, id: UUID) -> Optional[ModelType]:
        """
        Get a record by ID.

        Args:
            id: Record UUID

        Returns:
            Model instance or None if not found
        """
        return self.db.query(self.model).filter(self.model.id == id).first()

    def get_or_404(self, id: UUID) -> ModelType:
        """
        Get a record by ID or raise exception.

        Args:
            id: Record UUID

        Returns:
            Model instance

        Raises:
            ValueError: If record not found
        """
        obj = self.get(id)
        if not obj:
            raise ValueError(f"{self.model.__name__} with id {id} not found")
        return obj

    def get_multi(self, ids: List[UUID]) -> List[ModelType]:
        """
        Get multiple records by IDs.

        Args:
            ids: List of record UUIDs

        Returns:
            List of model instances
        """
        return self.db.query(self.model).filter(self.model.id.in_(ids)).all()

    def list(
        self,
        *,
        skip: int = 0,
        limit: int = 100,
        order_by: Optional[str] = None,
        order_desc: bool = True,
        filters: Optional[Dict[str, Any]] = None,
    ) -> List[ModelType]:
        """
        List records with optional filtering and pagination.

        Args:
            skip: Number of records to skip
            limit: Maximum records to return
            order_by: Field to order by (default: created_at)
            order_desc: Whether to order descending
            filters: Dictionary of field filters

        Returns:
            List of model instances
        """
        query = self.db.query(self.model)

        # Apply filters
        if filters:
            query = self._apply_filters(query, filters)

        # Apply ordering
        if order_by and hasattr(self.model, order_by):
            order_column = getattr(self.model, order_by)
            query = query.order_by(desc(order_column) if order_desc else asc(order_column))
        elif hasattr(self.model, "created_at"):
            query = query.order_by(desc(self.model.created_at) if order_desc else asc(self.model.created_at))

        return query.offset(skip).limit(limit).all()

    def count(self, filters: Optional[Dict[str, Any]] = None) -> int:
        """
        Count records with optional filtering.

        Args:
            filters: Dictionary of field filters

        Returns:
            Count of matching records
        """
        query = self.db.query(func.count(self.model.id))

        if filters:
            query = self._apply_filters(query, filters)

        return query.scalar()

    def update(
        self,
        id: UUID,
        data: Union[Dict[str, Any], Any],
        exclude_unset: bool = True,
    ) -> Optional[ModelType]:
        """
        Update a record by ID.

        Args:
            id: Record UUID
            data: Dictionary or Pydantic model with updated values
            exclude_unset: Whether to exclude unset fields

        Returns:
            Updated model instance or None if not found
        """
        db_obj = self.get(id)
        if not db_obj:
            return None

        if hasattr(data, "model_dump"):
            update_data = data.model_dump(exclude_unset=exclude_unset)
        elif hasattr(data, "dict"):
            update_data = data.dict(exclude_unset=exclude_unset)
        else:
            update_data = dict(data)

        for field, value in update_data.items():
            if hasattr(db_obj, field) and value is not None:
                setattr(db_obj, field, value)

        self.db.commit()
        self.db.refresh(db_obj)
        return db_obj

    def delete(self, id: UUID) -> bool:
        """
        Delete a record by ID.

        Args:
            id: Record UUID

        Returns:
            True if deleted, False if not found
        """
        db_obj = self.get(id)
        if not db_obj:
            return False

        self.db.delete(db_obj)
        self.db.commit()
        return True

    def soft_delete(self, id: UUID) -> Optional[ModelType]:
        """
        Soft delete a record (set is_deleted=True if supported).

        Args:
            id: Record UUID

        Returns:
            Updated model instance or None
        """
        db_obj = self.get(id)
        if not db_obj:
            return None

        if hasattr(db_obj, "is_deleted"):
            db_obj.is_deleted = True
            if hasattr(db_obj, "deleted_at"):
                db_obj.deleted_at = datetime.utcnow()
            self.db.commit()
            self.db.refresh(db_obj)
            return db_obj

        return None

    def exists(self, id: UUID) -> bool:
        """
        Check if a record exists.

        Args:
            id: Record UUID

        Returns:
            True if exists
        """
        return self.db.query(
            self.db.query(self.model).filter(self.model.id == id).exists()
        ).scalar()

    def _apply_filters(self, query, filters: Dict[str, Any]):
        """
        Apply filters to a query.

        Supports:
        - Exact match: {"field": value}
        - List match (IN): {"field": [value1, value2]}
        - Comparison: {"field__gt": value, "field__lt": value, "field__gte": value, "field__lte": value}
        - Like: {"field__like": "pattern%"}
        - Null check: {"field__isnull": True/False}
        """
        conditions = []

        for key, value in filters.items():
            if value is None:
                continue

            # Handle comparison operators
            if "__" in key:
                field_name, operator = key.rsplit("__", 1)
                if not hasattr(self.model, field_name):
                    continue

                column = getattr(self.model, field_name)

                if operator == "gt":
                    conditions.append(column > value)
                elif operator == "gte":
                    conditions.append(column >= value)
                elif operator == "lt":
                    conditions.append(column < value)
                elif operator == "lte":
                    conditions.append(column <= value)
                elif operator == "like":
                    conditions.append(column.like(value))
                elif operator == "ilike":
                    conditions.append(column.ilike(value))
                elif operator == "isnull":
                    if value:
                        conditions.append(column.is_(None))
                    else:
                        conditions.append(column.isnot(None))
                elif operator == "in":
                    conditions.append(column.in_(value))
            else:
                # Exact match or IN
                if not hasattr(self.model, key):
                    continue

                column = getattr(self.model, key)
                if isinstance(value, list):
                    conditions.append(column.in_(value))
                else:
                    conditions.append(column == value)

        if conditions:
            query = query.filter(and_(*conditions))

        return query

    def bulk_create(self, items: List[Union[Dict[str, Any], Any]]) -> List[ModelType]:
        """
        Create multiple records at once.

        Args:
            items: List of dictionaries or Pydantic models

        Returns:
            List of created model instances
        """
        db_objects = []
        for item in items:
            if hasattr(item, "model_dump"):
                obj_data = item.model_dump(exclude_unset=True)
            elif hasattr(item, "dict"):
                obj_data = item.dict(exclude_unset=True)
            else:
                obj_data = dict(item)

            db_objects.append(self.model(**obj_data))

        self.db.add_all(db_objects)
        self.db.commit()

        for obj in db_objects:
            self.db.refresh(obj)

        return db_objects

    def bulk_delete(self, ids: List[UUID]) -> int:
        """
        Delete multiple records by IDs.

        Args:
            ids: List of record UUIDs

        Returns:
            Number of deleted records
        """
        result = self.db.query(self.model).filter(self.model.id.in_(ids)).delete(
            synchronize_session=False
        )
        self.db.commit()
        return result

    def search(
        self,
        search_query: str,
        search_fields: List[str],
        skip: int = 0,
        limit: int = 100,
    ) -> List[ModelType]:
        """
        Search records across multiple fields.

        Args:
            search_query: Search string
            search_fields: List of field names to search
            skip: Number of records to skip
            limit: Maximum records to return

        Returns:
            List of matching records
        """
        if not search_query or not search_fields:
            return self.list(skip=skip, limit=limit)

        conditions = []
        search_pattern = f"%{search_query}%"

        for field in search_fields:
            if hasattr(self.model, field):
                column = getattr(self.model, field)
                conditions.append(column.ilike(search_pattern))

        if not conditions:
            return []

        query = self.db.query(self.model).filter(or_(*conditions))

        if hasattr(self.model, "created_at"):
            query = query.order_by(desc(self.model.created_at))

        return query.offset(skip).limit(limit).all()
